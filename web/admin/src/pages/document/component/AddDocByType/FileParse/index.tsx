import Upload from '@/components/UploadFile/Drag';
import {
  ConstsCrawlerSource,
  postApiV1CrawlerParse,
  postApiV1FileUpload,
} from '@/request';
import { useAppSelector } from '@/store';
import { formatByte } from '@/utils';
import { alpha, Box, CircularProgress, Stack, useTheme } from '@mui/material';
import { useCallback, useMemo, useState } from 'react';
import { v4 as uuidv4 } from 'uuid';
import { ListDataItem } from '..';
import { NoParseTypes, TYPE_CONFIG } from '../constants';
import { flattenCrawlerParseResponse } from '../util';

interface FileParseProps {
  type: ConstsCrawlerSource;
  parent_id: string | null;
  setData: React.Dispatch<React.SetStateAction<ListDataItem[]>>;
}

const FileParse = ({ type, parent_id, setData }: FileParseProps) => {
  const { kb_id } = useAppSelector(state => state.config);
  const theme = useTheme();
  const [loading, setLoading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [fileList, setFileList] = useState<File[]>([]);

  const isMultiple = useMemo(() => {
    return NoParseTypes.includes(type);
  }, [type]);

  const handleInitFiles = useCallback(
    async (uploadFiles: File[]) => {
      if (NoParseTypes.includes(type)) {
        const newFileList: ListDataItem[] = uploadFiles.map(file => ({
          uuid: uuidv4(),
          title: file.name,
          summary: formatByte(file.size),
          fileData: file,
          file: true,
          open: false,
          progress: 0,
          parent_id: parent_id || '',
          status: 'common' as const,
        }));
        setData(newFileList);
      } else {
        setFileList(uploadFiles);
        setLoading(true);
        const resp = await postApiV1FileUpload(
          { file: uploadFiles[0] },
          {
            onUploadProgress: progressEvent => {
              const percentCompleted = progressEvent.total
                ? Math.round((progressEvent.loaded * 100) / progressEvent.total)
                : 0;
              setProgress(percentCompleted);
            },
          },
        );
        const { key, filename } = resp;
        const parseResp = await postApiV1CrawlerParse({
          crawler_source: type,
          key,
          kb_id,
          filename,
        });
        const flattenedData = flattenCrawlerParseResponse(parseResp, parent_id);
        setData(prev => [...prev, ...flattenedData]);
      }
    },
    [type, parent_id],
  );

  return (
    <Box>
      {loading && fileList.length > 0 ? (
        <Stack
          direction='row'
          alignItems='center'
          justifyContent={'space-between'}
          gap={1}
          sx={{
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1,
            p: 1,
            px: 2,
            gap: 1,
            position: 'relative',
          }}
        >
          {progress && progress > 0 && progress < 100 ? (
            <Box
              sx={{
                width: `${progress}%`,
                transition: 'width 0.1s ease',
                height: '100%',
                backgroundColor: alpha(theme.palette.primary.main, 0.1),
                position: 'absolute',
                top: 0,
                left: 0,
              }}
            />
          ) : null}
          <Stack>
            <Box sx={{ fontSize: 14, color: 'text.primary' }}>
              {fileList[0].name}
            </Box>
            <Box sx={{ fontSize: 12, color: 'text.disabled' }}>
              {formatByte(fileList[0].size)}
            </Box>
          </Stack>
          <Stack direction={'row'} alignItems={'center'} gap={1}>
            <CircularProgress size={14} />
            <Box sx={{ fontSize: 12, color: 'text.disabled' }}>{progress}%</Box>
          </Stack>
        </Stack>
      ) : (
        <Upload
          accept={TYPE_CONFIG[type].accept}
          multiple={isMultiple}
          type={'drag'}
          onChange={handleInitFiles}
        />
      )}
    </Box>
  );
};
export default FileParse;
