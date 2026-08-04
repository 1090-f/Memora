import { KnowledgeBaseListItem } from '@/api';
import { deleteApiV1KnowledgeBaseDetail } from '@/request/KnowledgeBase';
import { useAppDispatch, useAppSelector } from '@/store';
import { setKbC, setKbId, setKbList } from '@/store/slices/config';
import ArrowForwardIosIcon from '@mui/icons-material/ArrowForwardIos';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import { Box, Stack, useTheme } from '@mui/material';
import { message, Modal } from '@ctzhian/ui';

interface KBDeleteProps {
  open: boolean;
  onClose: () => void;
  data: KnowledgeBaseListItem | null;
}

const KBDelete = ({ open, onClose, data }: KBDeleteProps) => {
  const theme = useTheme();
  const dispatch = useAppDispatch();
  const { kb_id, kbList } = useAppSelector(state => state.config);

  const handleOk = () => {
    if (!data) return;
    deleteApiV1KnowledgeBaseDetail({ id: data?.id || '' }).then(() => {
      message.success('删除成功');
      const newKbList = kbList?.filter(item => item.id !== data.id) || [];
      dispatch(setKbList(newKbList));
      if (kb_id === data.id && newKbList!.length > 0) {
        dispatch(setKbId(newKbList![0].id));
      }
      if (kbList!.length === 1) {
        dispatch(setKbC(true));
      }
      onClose();
    });
  };

  return (
    <Modal
      open={open}
      onCancel={() => {
        onClose();
      }}
      onOk={handleOk}
      okButtonProps={{ sx: { bgcolor: 'error.main' } }}
      title={
        <Stack direction='row' alignItems='center' gap={1}>
          <ErrorOutlineIcon sx={{ color: 'warning.main' }} />
          确定要删除该 Wiki 站吗？
        </Stack>
      }
    >
      <Stack
        direction='row'
        gap={2}
        sx={{
          borderBottom: '1px solid',
          borderColor: theme.palette.divider,
          py: 1,
        }}
      >
        <ArrowForwardIosIcon sx={{ fontSize: 12, mt: '4px', ml: 1 }} />
        <Box>{data?.name}</Box>
      </Stack>
    </Modal>
  );
};

export default KBDelete;
