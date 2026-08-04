import { putApiV1NodeDetail } from '@/request/Node';
import {
  createNodeSummaryStream,
  subscribeNodeSummaryStream,
  type StreamSummaryEvent,
} from '@/request/nodeStream';
import { DomainNodeListItemResp } from '@/request/types';
import { Button, Stack, TextField } from '@mui/material';
import { message, Modal } from '@ctzhian/ui';
import { useEffect, useRef, useState } from 'react';
import { IconShuaxin } from '@panda-wiki/icons';
import SSEClient from '@/utils/fetch';

interface SummaryProps {
  kb_id: string;
  data: DomainNodeListItemResp;
  open: boolean;
  refresh?: (value?: string) => void;
  onClose: () => void;
}

const Summary = ({ open, data, kb_id, onClose, refresh }: SummaryProps) => {
  const [generating, setGenerating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [summary, setSummary] = useState('');
  const sseClientRef = useRef<SSEClient<StreamSummaryEvent> | null>(null);

  const createSummary = () => {
    setGenerating(true);
    setSummary('');
    sseClientRef.current?.unsubscribe();
    sseClientRef.current = createNodeSummaryStream({
      onComplete: () => setGenerating(false),
      onError: error => {
        setGenerating(false);
        message.error(error.message || '生成摘要失败');
      },
    });
    subscribeNodeSummaryStream(
      sseClientRef.current,
      { kb_id, ids: [data.id!] },
      event => {
        if (event.type === 'data') {
          setSummary(prev => prev + (event.content || ''));
          return;
        }
        if (event.type === 'error') {
          setGenerating(false);
          message.error(event.content || event.error || '生成摘要失败');
          sseClientRef.current?.unsubscribe();
        }
      },
    );
  };

  const handleOk = () => {
    setSaving(true);
    putApiV1NodeDetail({
      id: data.id!,
      kb_id,
      nav_id: data.nav_id || '',
      summary,
    })
      .then(() => {
        message.success('保存成功');
        refresh?.(summary);
        onClose();
      })
      .finally(() => {
        setSaving(false);
      });
  };

  useEffect(() => {
    if (open) {
      setSummary(data.summary || '');
    }
  }, [open, data]);

  useEffect(() => {
    return () => {
      sseClientRef.current?.unsubscribe();
    };
  }, []);

  return (
    <Modal
      open={open}
      onCancel={() => {
        sseClientRef.current?.unsubscribe();
        onClose();
      }}
      disableEscapeKeyDown
      title={'文档摘要'}
      onOk={handleOk}
      okText='保存'
      okButtonProps={{ loading: saving, disabled: generating || saving }}
      footer={
        <Stack
          direction={'row'}
          alignItems={'center'}
          justifyContent={'space-between'}
          sx={{ p: 3, pt: 0 }}
        >
          <Button
            sx={{ minWidth: 'auto' }}
            onClick={createSummary}
            disabled={generating || saving}
            startIcon={
              <IconShuaxin
                sx={{
                  fontSize: '16px !important',
                  ...(generating
                    ? { animation: 'loadingRotate 1s linear infinite' }
                    : {}),
                }}
              />
            }
          >
            AI 生成
          </Button>
          <Stack direction={'row'} alignItems={'center'} gap={2}>
            <Button
              onClick={() => {
                sseClientRef.current?.unsubscribe();
                onClose();
              }}
              sx={{ color: 'text.primary' }}
            >
              取消
            </Button>
            <Button
              sx={{ width: 100 }}
              loading={saving}
              onClick={handleOk}
              disabled={generating || saving}
              variant='contained'
            >
              保存
            </Button>
          </Stack>
        </Stack>
      }
    >
      <TextField
        autoFocus
        fullWidth
        multiline
        minRows={6}
        maxRows={12}
        value={summary}
        placeholder='暂无摘要，可在此处编辑'
        disabled={generating || saving}
        onChange={event => {
          setSummary(event.target.value);
        }}
      />
    </Modal>
  );
};

export default Summary;
