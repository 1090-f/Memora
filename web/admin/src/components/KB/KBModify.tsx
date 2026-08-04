import { KnowledgeBaseListItem, updateKnowledgeBase } from '@/api';
import { useAppDispatch, useAppSelector } from '@/store';
import { setKbList } from '@/store/slices/config';
import { message, Modal } from '@ctzhian/ui';
import { TextField } from '@mui/material';
import { useEffect, useState } from 'react';

interface KBModifyProps {
  open: boolean;
  data: KnowledgeBaseListItem | null;
  onClose: () => void;
}

const KBModify = ({ open, data, onClose }: KBModifyProps) => {
  const [kbName, setKbName] = useState(data?.name || '');
  const { kbList } = useAppSelector(state => state.config);
  const dispatch = useAppDispatch();

  const handleClose = () => {
    setKbName(data?.name || '');
    onClose();
  };

  const handleSave = () => {
    if (!data?.id) return;
    if (!kbName) {
      message.warning('请输入知识库名称');
      return;
    }
    updateKnowledgeBase({ id: data.id, name: kbName }).then(() => {
      message.success('保存成功');
      dispatch(
        setKbList(
          kbList?.map(item =>
            item.id === data.id ? { ...item, name: kbName } : item,
          ),
        ),
      );
      onClose();
    });
  };

  useEffect(() => {
    setKbName(data?.name || '');
  }, [data]);

  return (
    <Modal
      title='修改知识库名称'
      open={open}
      okText='保存'
      cancelText='取消'
      onOk={handleSave}
      onCancel={handleClose}
    >
      <TextField
        fullWidth
        value={kbName}
        placeholder='请输入知识库名称'
        onChange={e => {
          setKbName(e.target.value);
        }}
      />
    </Modal>
  );
};

export default KBModify;
