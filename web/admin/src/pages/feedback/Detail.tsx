import { ChatConversationPair } from '@/api';
import { getApiV1ConversationMessageDetail } from '@/request';
import MarkDown from '@/components/MarkDown';
import { useAppSelector } from '@/store';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import { Box, Stack, Typography, alpha } from '@mui/material';
import { Ellipsis, Modal } from '@ctzhian/ui';
import { useEffect, useState } from 'react';
import {
  StyledConversationItem,
  StyledUserBubble,
  StyledAiBubble,
  StyledThinkingAccordion,
  StyledThinkingAccordionSummary,
  StyledThinkingAccordionDetails,
  StyledAiBubbleContent,
} from '../conversation/Detail';

const Detail = ({
  id,
  open,
  onClose,
  data,
}: {
  id: string;
  open: boolean;
  data: any;
  onClose: () => void;
}) => {
  const [conversations, setConversations] = useState<Omit<
    ChatConversationPair,
    'info' | 'image_paths'
  > | null>(null);
  const { kb_id = '' } = useAppSelector(state => state.config);

  useEffect(() => {
    if (open && id && data) {
      getApiV1ConversationMessageDetail({ id, kb_id }).then(res => {
        setConversations({
          user: data.question,
          assistant: res.content!,
          created_at: res.created_at!,
          thinking_content: '',
        });
      });
    }
  }, [open, data, id]);

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
      <Box sx={{ fontSize: 14 }}>
        <Box>
          <StyledConversationItem>
            {/* 用户问题气泡 - 右对齐 */}
            <StyledUserBubble>{conversations?.user}</StyledUserBubble>

            {/* AI回答气泡 - 左对齐 */}
            <StyledAiBubble>
              {/* 思考过程 */}
              {!!conversations?.thinking_content && (
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
                    <MarkDown content={conversations?.thinking_content || ''} />
                  </StyledThinkingAccordionDetails>
                </StyledThinkingAccordion>
              )}

              {/* AI回答内容 */}
              <StyledAiBubbleContent>
                <MarkDown content={conversations?.assistant || ''} />
              </StyledAiBubbleContent>
            </StyledAiBubble>
          </StyledConversationItem>
        </Box>
      </Box>
    </Modal>
  );
};

export default Detail;
