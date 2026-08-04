import { ChatConversationPair } from '@/api';
import { getApiV1ConversationDetail } from '@/request/Conversation';
import { DomainConversationDetailResp } from '@/request/types';
import Avatar from '@/components/Avatar';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import Card from '@/components/Card';
import MarkDown from '@/components/MarkDown';
import { useAppSelector } from '@/store';
import { getBasePath } from '@/utils/getBasePath';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Stack,
  useTheme,
  styled,
  alpha,
  Typography,
} from '@mui/material';
import { Ellipsis, Modal, Image } from '@ctzhian/ui';
import { useEffect, useState } from 'react';
import { IconDitu_diqiu } from '@panda-wiki/icons';

const handleThinkingContent = (content: string) => {
  const thinkRegex = /<think>([\s\S]*?)(?:<\/think>|$)/g;
  const thinkMatches = [];
  let match;
  while ((match = thinkRegex.exec(content)) !== null) {
    thinkMatches.push(match[1]);
  }

  let answerContent = content.replace(/<think>[\s\S]*?<\/think>/g, '');
  answerContent = answerContent.replace(/<think>[\s\S]*$/, '');

  return {
    thinkingContent: thinkMatches.join(''),
    answerContent: answerContent,
  };
};

export const StyledConversationItem = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(2),
}));

// 聊天气泡相关组件
export const StyledUserBubble = styled(Box)(({ theme }) => ({
  alignSelf: 'flex-end',
  maxWidth: '75%',
  padding: theme.spacing(1, 2),
  borderRadius: '10px 10px 0px 10px',
  backgroundColor: theme.palette.primary.main,
  color: theme.palette.primary.contrastText,
  fontSize: 14,
  wordBreak: 'break-word',
}));

export const StyledAiBubble = styled(Box)(({ theme }) => ({
  alignSelf: 'flex-start',
  display: 'flex',
  flexDirection: 'column',
  width: '100%',
  gap: theme.spacing(3),
}));

export const StyledAiBubbleContent = styled(Box)(() => ({
  wordBreak: 'break-word',
}));

// 对话相关组件
export const StyledAccordion = styled(Accordion)(() => ({
  padding: 0,
  border: 'none',
  '&:before': {
    content: '""',
    height: 0,
  },
  background: 'transparent',
  backgroundImage: 'none',
}));

export const StyledAccordionSummary = styled(AccordionSummary)(({ theme }) => ({
  paddingLeft: theme.spacing(2),
  paddingRight: theme.spacing(2),
  paddingTop: theme.spacing(1),
  paddingBottom: theme.spacing(1),
  userSelect: 'text',
  borderRadius: '10px',
  backgroundColor: theme.palette.background.paper3,
  border: '1px solid',
  borderColor: theme.palette.divider,
}));

export const StyledAccordionDetails = styled(AccordionDetails)(({ theme }) => ({
  padding: theme.spacing(2),
  borderTop: 'none',
}));

export const StyledQuestionText = styled(Box)(() => ({
  fontWeight: '700',
  fontSize: 16,
  lineHeight: '24px',
  wordBreak: 'break-all',
}));

// 搜索结果相关组件
export const StyledChunkAccordion = styled(Accordion)(({ theme }) => ({
  backgroundImage: 'none',
  background: 'transparent',
  border: 'none',
  padding: 0,
}));

export const StyledChunkAccordionSummary = styled(AccordionSummary)(
  ({ theme }) => ({
    justifyContent: 'flex-start',
    gap: theme.spacing(2),
    '.MuiAccordionSummary-content': {
      flexGrow: 0,
    },
  }),
);

export const StyledChunkAccordionDetails = styled(AccordionDetails)(
  ({ theme }) => ({
    paddingTop: 0,
    paddingLeft: theme.spacing(2),
    borderTop: 'none',
    borderLeft: '1px solid',
    borderColor: theme.palette.divider,
  }),
);

export const StyledChunkItem = styled(Box)(({ theme }) => ({
  cursor: 'pointer',
  '&:hover': {
    '.hover-primary': {
      color: theme.palette.primary.main,
    },
  },
}));

// 思考过程相关组件
export const StyledThinkingAccordion = styled(Accordion)(({ theme }) => ({
  backgroundColor: 'transparent',
  border: 'none',
  padding: 0,
  paddingBottom: theme.spacing(2),
  '&:before': {
    content: '""',
    height: 0,
  },
}));

export const StyledThinkingAccordionSummary = styled(AccordionSummary)(
  ({ theme }) => ({
    justifyContent: 'flex-start',
    gap: theme.spacing(2),
    '.MuiAccordionSummary-content': {
      flexGrow: 0,
    },
  }),
);

export const StyledThinkingAccordionDetails = styled(AccordionDetails)(
  ({ theme }) => ({
    paddingTop: 0,
    paddingLeft: theme.spacing(2),
    borderTop: 'none',
    borderLeft: '1px solid',
    borderColor: theme.palette.divider,
    '.markdown-body': {
      opacity: 0.75,
      fontSize: 12,
    },
  }),
);

const Detail = ({
  id,
  open,
  onClose,
}: {
  id: string;
  open: boolean;
  onClose: () => void;
}) => {
  const theme = useTheme();
  const { kb_id = '' } = useAppSelector(state => state.config);
  const [detail, setDetail] = useState<DomainConversationDetailResp | null>(
    null,
  );
  const [conversations, setConversations] = useState<
    ChatConversationPair[] | null
  >(null);

  const getDetail = () => {
    getApiV1ConversationDetail({ id, kb_id }).then(res => {
      setDetail(res);
      const pairs: ChatConversationPair[] = [];
      let currentPair: Partial<ChatConversationPair> = {};
      res.messages?.forEach(message => {
        if (message.role === 'user') {
          currentPair = {
            user: message.content,
            image_paths: message.image_paths,
          };
        } else if (message.role === 'assistant') {
          if (
            currentPair.user ||
            (currentPair.image_paths && currentPair.image_paths.length > 0)
          ) {
            const { thinkingContent, answerContent } = handleThinkingContent(
              message.content || '',
            );
            currentPair.assistant = answerContent;
            currentPair.thinking_content = thinkingContent;
            currentPair.created_at = message.created_at;
            // @ts-expect-error 类型不兼容
            currentPair.info = message.info;
            pairs.push(currentPair as ChatConversationPair);
            currentPair = {};
          }
        }
      });

      if (
        currentPair.user ||
        (currentPair.image_paths && currentPair.image_paths.length > 0)
      ) {
        pairs.push({
          user: currentPair.user,
          image_paths: currentPair.image_paths,
          assistant: '',
          created_at: '',
          info: { score: 0 },
        } as ChatConversationPair);
      }

      setConversations(pairs);
    });
  };

  useEffect(() => {
    if (open && id) getDetail();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, open]);

  return (
    <Modal
      title={
        <Ellipsis
          sx={{
            fontWeight: 'bold',
            fontSize: 20,
            lineHeight: '22px',
            width: 700,
          }}
        >
          问答记录
        </Ellipsis>
      }
      width={800}
      open={open}
      onCancel={onClose}
      footer={null}
    >
      {detail ? (
        <Box sx={{ fontSize: 14 }}>
          {(detail.references?.length || 0) > 0 && (
            <>
              <Stack
                direction={'row'}
                alignItems={'center'}
                gap={1}
                sx={{
                  fontWeight: 'bold',
                  mt: 2,
                  mb: 1,
                  '&::before': {
                    content: '""',
                    display: 'inline-block',
                    width: '4px',
                    height: '12px',
                    borderRadius: '2px',
                    backgroundColor: theme.palette.primary.main,
                  },
                }}
              >
                内容来源
              </Stack>
              <Card sx={{ p: 2, bgcolor: 'background.paper3' }}>
                {detail.references?.map((item, index) => (
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    key={index}
                  >
                    <Avatar
                      // @ts-expect-error 类型不兼容
                      src={item.favicon}
                      sx={{ width: 18, height: 18 }}
                      errorIcon={
                        <IconDitu_diqiu
                          sx={{ fontSize: 18, color: 'text.tertiary' }}
                        />
                      }
                    />
                    <Ellipsis>
                      <Box
                        component={'a'}
                        href={item.url}
                        target='_blank'
                        sx={{
                          color: 'text.primary',
                          '&:hover': { color: 'primary.main' },
                        }}
                      >
                        {/* @ts-expect-error 类型不兼容 */}
                        {item.title}
                      </Box>
                    </Ellipsis>
                  </Stack>
                ))}
              </Card>
            </>
          )}
          <Stack gap={2}>
            {conversations &&
              conversations.map((item, index) => (
                <StyledConversationItem key={index}>
                  {item.image_paths && item.image_paths.length > 0 && (
                    <Image.PreviewGroup>
                      <Stack
                        direction='row'
                        gap={1}
                        sx={{ alignSelf: 'flex-end' }}
                      >
                        {item.image_paths.map((url: string) => (
                          <Image
                            key={url}
                            alt={url}
                            src={getBasePath(url)}
                            width={100}
                            height={100}
                            style={{
                              borderRadius: '10px',
                              objectFit: 'cover',
                              cursor: 'pointer',
                            }}
                            referrerPolicy='no-referrer'
                          />
                        ))}
                      </Stack>
                    </Image.PreviewGroup>
                  )}
                  {/* 用户问题气泡 - 右对齐 */}
                  {item.user && (
                    <StyledUserBubble>{item.user}</StyledUserBubble>
                  )}

                  {/* AI回答气泡 - 左对齐 */}
                  <StyledAiBubble>
                    {/* 思考过程 */}
                    {!!item.thinking_content && (
                      <StyledThinkingAccordion defaultExpanded>
                        <StyledThinkingAccordionSummary
                          expandIcon={<ExpandMoreIcon sx={{ fontSize: 16 }} />}
                        >
                          <Stack direction='row' alignItems='center' gap={1}>
                            <Typography
                              variant='body2'
                              sx={theme => ({
                                fontSize: 12,
                                color: alpha(theme.palette.text.primary, 0.5),
                              })}
                            >
                              已思考
                            </Typography>
                          </Stack>
                        </StyledThinkingAccordionSummary>

                        <StyledThinkingAccordionDetails>
                          <MarkDown content={item.thinking_content || ''} />
                        </StyledThinkingAccordionDetails>
                      </StyledThinkingAccordion>
                    )}

                    {/* AI回答内容 */}
                    <StyledAiBubbleContent>
                      <MarkDown content={item.assistant} />
                    </StyledAiBubbleContent>
                  </StyledAiBubble>
                </StyledConversationItem>
              ))}
          </Stack>
        </Box>
      ) : (
        <Box></Box>
      )}
    </Modal>
  );
};

export default Detail;
