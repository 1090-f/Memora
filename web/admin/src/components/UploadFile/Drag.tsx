import { Upload as UploadIcon } from '@mui/icons-material';
import { Box, Button, Stack, Typography, useTheme } from '@mui/material';
import { useCallback, useEffect, useRef, useState } from 'react';
import { FileRejection, useDropzone } from 'react-dropzone';
import { IconShangchuan } from '@panda-wiki/icons';

// 文件扩展名到 MIME 类型的映射
const FILE_EXTENSION_TO_MIME: Record<string, string> = {
  // 文本文件
  '.txt': 'text/plain',
  '.md': 'text/markdown',
  '.html': 'text/html',
  // Office 文档
  '.xls': 'application/vnd.ms-excel',
  '.xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  '.docx':
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  '.pptx':
    'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  '.pdf': 'application/pdf',
  // 压缩文件
  '.zip': 'application/zip',
  // 电子书
  '.epub': 'application/epub+zip',
  // 知识库导出文件
  '.lakebook': 'application/octet-stream',
};

interface UploadProps {
  file?: File[];
  onChange: (acceptedFiles: File[], rejectedFiles: FileRejection[]) => void;
  type?: 'drag' | 'select';
  accept?: string;
  acceptDisplay?: string; // 用于页面显示的文件格式文本
  size?: number;
  multiple?: boolean;
}

const Upload = ({
  file,
  onChange,
  type = 'select',
  accept,
  acceptDisplay,
  multiple = true,
}: UploadProps) => {
  const theme = useTheme();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [dropFiles, setDropFiles] = useState<File[]>(file || []);

  const onDrop = useCallback(
    (acceptedFiles: File[], rejectedFiles: FileRejection[]) => {
      const validFiles = acceptedFiles;

      const newFiles = multiple ? [...(file || []), ...validFiles] : validFiles;
      setDropFiles(newFiles);
      onChange(newFiles, rejectedFiles);
    },
    [dropFiles, onChange, multiple],
  );

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    accept: accept
      ? accept.split(',').reduce((acc: Record<string, string[]>, item) => {
          const trimmedItem = item.trim();
          if (trimmedItem) {
            // 如果是文件扩展名（以 . 开头），转换为 MIME 类型
            if (trimmedItem.startsWith('.')) {
              const mimeType = FILE_EXTENSION_TO_MIME[trimmedItem];
              if (mimeType) {
                acc[mimeType] = [];
              }
            } else {
              // 否则直接作为 MIME 类型使用
              acc[trimmedItem] = [];
            }
          }
          return acc;
        }, {})
      : undefined,
    multiple,
    noClick: type === 'drag',
    noKeyboard: type === 'drag',
  });

  useEffect(() => {
    if (file) setDropFiles(file);
  }, [file]);

  return (
    <Box sx={{ width: '100%' }}>
      {type === 'drag' && (
        <Stack
          alignItems='center'
          {...getRootProps()}
          sx={{
            border: `1px solid ${theme.palette.divider}`,
            borderRadius: '10px',
            p: 4,
            textAlign: 'center',
            backgroundColor: isDragActive
              ? 'primary.main'
              : 'background.paper3',
            cursor: 'pointer',
            '&:hover': {
              borderColor: theme.palette.primary.main,
            },
          }}
          onClick={() => fileInputRef.current?.click()}
        >
          <input {...getInputProps()} />
          <IconShangchuan
            sx={{ fontSize: 40, mb: 1, color: 'text.secondary' }}
          />
          <Typography
            variant='body1'
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 13,
            }}
          >
            <Button
              variant='text'
              sx={{
                fontSize: 13,
                minWidth: 'auto',
                p: 0,
                ml: 1,
                lineHeight: 1,
              }}
            >
              点击浏览文件
            </Button>
            或拖拽文件到区域内
          </Typography>
          <Typography
            variant='body2'
            color='text.secondary'
            sx={{ mt: 1, fontSize: 12 }}
          >
            支持格式 {acceptDisplay || accept || '所有文件'}
          </Typography>
          {/* {size && <Typography variant='body2' color='text.secondary' sx={{ mt: 0.5, fontSize: 12 }}>
            支持上传大小不超过 {formatByte(size)} 的文件
          </Typography>} */}
        </Stack>
      )}

      {/* 普通选择按钮 */}
      {type === 'select' && (
        <Button
          variant='outlined'
          startIcon={<UploadIcon />}
          onClick={() => fileInputRef.current?.click()}
        >
          选择文件
        </Button>
      )}

      <input
        type='file'
        ref={fileInputRef}
        hidden
        accept={accept}
        multiple={multiple}
        onChange={e => {
          if (e.target.files) {
            onDrop(Array.from(e.target.files), []);
            // 清空 input value，以便能够选择相同的文件
            e.target.value = '';
          }
        }}
      />
    </Box>
  );
};

export default Upload;
