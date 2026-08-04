import { UserInfo } from '@/api';
import { deleteApiV1UserDelete } from '@/request/User';
import Card from '@/components/Card';
import ArrowForwardIosIcon from '@mui/icons-material/ArrowForwardIos';
import { Box, Stack } from '@mui/material';
import { Ellipsis, message, Modal } from '@ctzhian/ui';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import { V1UserListItemResp } from '@/request/types';

interface MemberDeleteProps {
  open: boolean;
  onClose: () => void;
  user: V1UserListItemResp | null;
  refresh: () => void;
}

const MemberDelete = ({ open, onClose, user, refresh }: MemberDeleteProps) => {
  const submit = () => {
    if (!user?.id) return;
    deleteApiV1UserDelete({ user_id: user.id }).then(() => {
      message.success('删除成功');
      refresh();
      onClose();
    });
  };

  if (!user) return null;
  return (
    <Modal
      open={open}
      width={600}
      okText='删除'
      okButtonProps={{ sx: { bgcolor: 'error.main' } }}
      onCancel={onClose}
      onOk={submit}
      title={
        <Stack direction='row' alignItems='center' gap={1}>
          <ErrorOutlineIcon sx={{ color: 'warning.main' }} />
          确定要删除该用户吗？{' '}
        </Stack>
      }
    >
      <Card
        sx={{
          fontSize: 14,
          p: 1,
          bgcolor: 'background.paper3',
        }}
      >
        <Stack
          direction='row'
          gap={2}
          sx={{
            borderBottom: '1px solid',
            borderColor: 'divider',
            py: 1,
          }}
        >
          <ArrowForwardIosIcon sx={{ fontSize: 12, mt: '4px' }} />
          <Box sx={{ width: '100%' }}>
            <Ellipsis sx={{ width: 'calc(100% - 30px)' }}>
              {user.account || '-'}
            </Ellipsis>
          </Box>
        </Stack>
      </Card>
    </Modal>
  );
};
export default MemberDelete;
