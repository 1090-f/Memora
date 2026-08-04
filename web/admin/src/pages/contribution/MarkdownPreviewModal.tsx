import {
  ConstsContributeStatus,
  ConstsContributeType,
  getApiProV1ContributeDetail,
  GithubComChaitinPandaWikiProApiContributeV1ContributeDetailResp,
  GithubComChaitinPandaWikiProApiContributeV1ContributeItem,
} from '@/request/pro';
import { useAppSelector } from '@/store';
import { Modal } from '@ctzhian/ui';
import { Box, Button, Stack } from '@mui/material';
import { IconWenjian } from '@panda-wiki/icons';
import dayjs from 'dayjs';
import { useEffect, useState } from 'react';
import ReactDiffViewer from 'react-diff-viewer';

type MarkdownPreviewModalProps = {
  open: boolean;
  row: GithubComChaitinPandaWikiProApiContributeV1ContributeItem | null;
  onClose: () => void;
  onAccept: () => void;
  onReject: () => void;
};

const MarkdownPreviewModal = ({
  open,
  row,
  onClose,
  onAccept,
  onReject,
}: MarkdownPreviewModalProps) => {
  const { kb_id = '' } = useAppSelector(state => state.config);
  const [data, setData] =
    useState<GithubComChaitinPandaWikiProApiContributeV1ContributeDetailResp | null>(
      null,
    );

  useEffect(() => {
    if (open && row) {
      getApiProV1ContributeDetail({ id: row.id!, kb_id }).then(res => {
        setData(res);
      });
    }
  }, [open, row, kb_id]);

  return (
    <Modal
      open={open}
      onCancel={onClose}
      width={'1200px'}
      sx={{
        '.MuiDialogContent-root': {
          display: 'flex',
          flexDirection: 'column',
        },
      }}
      title={
        <Stack direction='row' alignItems='center' gap={2}>
          <Box>
            来自 {row?.auth_name || '匿名用户'} 的
            {row?.type === ConstsContributeType.ContributeTypeAdd
              ? '新增'
              : '修改'}
          </Box>
          <Box sx={{ fontSize: 14, color: 'text.tertiary', fontWeight: 400 }}>
            {dayjs(row?.created_at).fromNow()}
          </Box>
        </Stack>
      }
      footer={
        row?.status === ConstsContributeStatus.ContributeStatusPending ||
        row?.status === ConstsContributeStatus.ContributeStatusRejected ? (
          <Stack
            direction='row'
            gap={1}
            justifyContent='flex-end'
            sx={{ p: 3, pt: 0 }}
          >
            {row?.status === ConstsContributeStatus.ContributeStatusPending ? (
              <>
                <Button
                  size='small'
                  variant='outlined'
                  color='error'
                  onClick={onReject}
                >
                  拒绝
                </Button>
                <Button size='small' variant='contained' onClick={onAccept}>
                  采纳
                </Button>
              </>
            ) : (
              <Button onClick={onClose} size='small' variant='contained'>
                关闭
              </Button>
            )}
          </Stack>
        ) : null
      }
    >
      <Stack direction='row'>
        <Stack
          spacing={2}
          sx={{
            overflow: 'auto',
            flex: 1,
          }}
        >
          <Stack
            direction='row'
            gap={1}
            sx={{ bgcolor: 'background.paper2', p: 1, borderRadius: '10px' }}
          >
            <Box sx={{ fontSize: 14, fontWeight: 'bold', flexShrink: 0 }}>
              提交说明：
            </Box>

            <Box sx={{ fontSize: 14, color: 'text.tertiary' }}>
              {data?.reason || '-'}
            </Box>
          </Stack>
          <Stack
            direction='row'
            alignItems='center'
            gap={1}
            sx={{ fontSize: 24, fontWeight: 700, pb: 2 }}
          >
            <IconWenjian /> {row?.node_name || '-'}
          </Stack>
          <Box sx={{ overflowY: 'auto', maxHeight: 'calc(100vh - 400px)' }}>
            <ReactDiffViewer
              oldValue={data?.original_node?.content || ''}
              newValue={data?.content || ''}
            />
          </Box>
        </Stack>
      </Stack>
    </Modal>
  );
};

export default MarkdownPreviewModal;
