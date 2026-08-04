import {
  createNodeSummaryStream,
  subscribeNodeSummaryStream,
  type StreamSummaryEvent,
} from '@/request/nodeStream';
import { DomainRecommendNodeListResp } from '@/request/types';
import { useAppSelector } from '@/store';
import { Box, IconButton, Stack } from '@mui/material';
import { Ellipsis, message } from '@ctzhian/ui';
import {
  IconShanchu2,
  IconDrag,
  IconWenjianjia,
  IconWenjian,
} from '@panda-wiki/icons';
import {
  CSSProperties,
  forwardRef,
  HTMLAttributes,
  useRef,
  useState,
} from 'react';
import SSEClient from '@/utils/fetch';

export type ItemProps = HTMLAttributes<HTMLDivElement> & {
  item: DomainRecommendNodeListResp;
  withOpacity?: boolean;
  isDragging?: boolean;
  dragHandleProps?: any;
  handleRemove?: (id: string) => void;
  refresh?: () => void;
};

const Item = forwardRef<HTMLDivElement, ItemProps>(
  (
    {
      item,
      withOpacity,
      isDragging,
      style,
      dragHandleProps,
      handleRemove,
      refresh,
      ...props
    },
    ref,
  ) => {
    const { kb_id } = useAppSelector(state => state.config);
    const inlineStyles: CSSProperties = {
      opacity: withOpacity ? '0.5' : '1',
      borderRadius: '10px',
      cursor: isDragging ? 'grabbing' : 'grab',
      backgroundColor: '#ffffff',
      width: '100%',
      minWidth: '0px',
      ...style,
    };
    const [loading, setLoading] = useState(false);
    const sseClientRef = useRef<SSEClient<StreamSummaryEvent> | null>(null);

    const handleCreateSummary = () => {
      setLoading(true);
      sseClientRef.current?.unsubscribe();
      sseClientRef.current = createNodeSummaryStream({
        onComplete: () => setLoading(false),
        onError: error => {
          setLoading(false);
          message.error(error.message || '生成摘要失败');
        },
      });
      subscribeNodeSummaryStream(
        sseClientRef.current,
        { ids: [item.id!], kb_id },
        event => {
          if (event.type === 'done') {
            setLoading(false);
            message.success('生成摘要成功');
            refresh?.();
            return;
          }
          if (event.type === 'error') {
            setLoading(false);
            message.error(event.content || event.error || '生成摘要失败');
            sseClientRef.current?.unsubscribe();
          }
        },
      );
    };

    const recommend_nodes = [...(item.recommend_nodes || [])];

    return (
      <Box ref={ref} style={inlineStyles} {...props}>
        <Stack
          direction={'row'}
          gap={1}
          sx={{
            p: 1,
            height: '100%',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: '10px',
          }}
        >
          <Box
            sx={{
              px: 2,
              py: 1,
              flexGrow: 1,
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: '10px',
            }}
          >
            <Stack direction={'row'} alignItems={'center'} gap={1}>
              {item.emoji ? (
                <Box sx={{ fontSize: 14, color: '#2f80f7', flexShrink: 0 }}>
                  {item.emoji}
                </Box>
              ) : item.type === 1 ? (
                <IconWenjianjia
                  sx={{ fontSize: 14, color: '#2f80f7', flexShrink: 0 }}
                />
              ) : (
                <IconWenjian
                  sx={{ fontSize: 14, color: '#2f80f7', flexShrink: 0 }}
                />
              )}
              <Ellipsis sx={{ flex: 1, width: 0, lineHeight: '32px' }}>
                {item.name}
              </Ellipsis>
            </Stack>
            {item.summary ? (
              <Box
                className='ellipsis-5'
                sx={{
                  fontSize: 14,
                  color: 'text.tertiary',
                  lineHeight: '21px',
                }}
              >
                {item.summary}
              </Box>
            ) : item.type === 2 ? (
              <Box
                sx={{ color: 'warning.main', fontSize: 12, lineHeight: '21px' }}
              >
                暂无摘要，可前往文档页生成并发布
              </Box>
            ) : null}
            {/* : item.type === 2 ? <Button size='small' loading={loading} sx={{
            height: '21px',
            px: 0,
            ml: '18px',
          }} onClick={handleCreateSummary}>生成摘要</Button> : null} */}
            {recommend_nodes.length > 0 && (
              <Stack sx={{ fontSize: 14, color: 'text.tertiary', pl: '20px' }}>
                {recommend_nodes
                  ?.sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
                  .slice(0, 4)
                  .map(it => (
                    <Stack
                      direction={'row'}
                      alignItems={'center'}
                      gap={1}
                      key={it.id}
                    >
                      {it.emoji ? (
                        <Box
                          sx={{ fontSize: 14, color: '#2f80f7', flexShrink: 0 }}
                        >
                          {it.emoji}
                        </Box>
                      ) : it.type === 1 ? (
                        <IconWenjianjia
                          sx={{ fontSize: 14, color: '#2f80f7', flexShrink: 0 }}
                        />
                      ) : (
                        <IconWenjian
                          sx={{ fontSize: 14, color: '#2f80f7', flexShrink: 0 }}
                        />
                      )}

                      <Ellipsis sx={{ flex: 1, width: 0 }}>{it.name}</Ellipsis>
                    </Stack>
                  ))}
              </Stack>
            )}
          </Box>
          <Stack justifyContent={'space-between'} sx={{ flexShrink: 0 }}>
            <IconButton
              size='small'
              onClick={e => {
                e.stopPropagation();
                handleRemove?.(item.id!);
              }}
              sx={{
                color: 'text.tertiary',
                ':hover': { color: 'error.main' },
                width: '28px',
                height: '28px',
              }}
            >
              <IconShanchu2 sx={{ fontSize: '12px' }} />
            </IconButton>

            <IconButton
              size='small'
              sx={{
                cursor: 'grab',
                color: 'text.secondary',
                '&:hover': { color: 'primary.main' },
              }}
              {...dragHandleProps}
            >
              <IconDrag sx={{ fontSize: '18px' }} />
            </IconButton>
          </Stack>
        </Stack>
      </Box>
    );
  },
);

export default Item;
