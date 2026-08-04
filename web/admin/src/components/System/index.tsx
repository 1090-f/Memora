import { getApiV1ModelList } from '@/request/Model';
import { GithubComChaitinPandaWikiDomainModelListItem } from '@/request/types';
import { useAppDispatch, useAppSelector } from '@/store';
import { setModelList, setModelStatus } from '@/store/slices/config';
import { Modal } from '@ctzhian/ui';
import { IconAChilunshezhisheding } from '@panda-wiki/icons';
import { Box, Button, Tab, Tabs, useTheme } from '@mui/material';
import { useEffect, useState, useRef } from 'react';

import Member from './component/Member';
import ModelConfig, { ModelConfigRef } from './component/ModelConfig';

const SystemTabs = [
  { label: '模型配置', id: 'model-config' },
  { label: '用户管理', id: 'user-management' },
];

const System = () => {
  const theme = useTheme();
  const { user, modelList, isCreateWikiModalOpen } = useAppSelector(
    state => state.config,
  );
  const [open, setOpen] = useState(false);
  const [activeTab, setActiveTab] = useState('model-config');
  const dispatch = useAppDispatch();
  const modelConfigRef = useRef<ModelConfigRef>(null);
  const [chatModelData, setChatModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);
  const [embeddingModelData, setEmbeddingModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);
  const [rerankModelData, setRerankModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);
  const [analysisModelData, setAnalysisModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);
  const [analysisVLModelData, setAnalysisVLModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);

  const getModelList = () => {
    getApiV1ModelList().then(res => {
      dispatch(
        setModelList(res as GithubComChaitinPandaWikiDomainModelListItem[]),
      );
    });
  };

  const handleModelList = (
    list: GithubComChaitinPandaWikiDomainModelListItem[],
  ) => {
    const chat = list.find(it => it.type === 'chat') || null;
    const embedding = list.find(it => it.type === 'embedding') || null;
    const rerank = list.find(it => it.type === 'rerank') || null;
    const analysis = list.find(it => it.type === 'analysis') || null;
    const analysisVL = list.find(it => it.type === 'analysis-vl') || null;
    setChatModelData(chat);
    setEmbeddingModelData(embedding);
    setRerankModelData(rerank);
    setAnalysisModelData(analysis);
    setAnalysisVLModelData(analysisVL);

    // 检查模型配置状态
    const status = !!(chat && embedding && rerank);
    dispatch(setModelStatus(status));
  };

  useEffect(() => {
    if (modelList) {
      handleModelList(modelList);
    }
  }, [modelList]);

  useEffect(() => {
    if (isCreateWikiModalOpen) {
      setOpen(false);
    }
  }, [isCreateWikiModalOpen]);

  return (
    <>
      <Box sx={{ position: 'relative' }}>
        {user.role === 'admin' && (
          <Button
            size='small'
            variant='outlined'
            startIcon={<IconAChilunshezhisheding />}
            onClick={() => setOpen(true)}
          >
            系统配置
          </Button>
        )}
      </Box>
      <Modal
        title='系统配置'
        width={1100}
        open={open}
        disableEnforceFocus={true}
        footer={null}
        onCancel={() => {
          if (activeTab === 'model-config' && modelConfigRef.current) {
            modelConfigRef.current.handleClose();
          } else {
            setOpen(false);
          }
        }}
      >
        <Tabs
          value={activeTab}
          onChange={(event, newValue) => setActiveTab(newValue)}
          aria-label='system tabs'
          sx={{
            mb: 2,
            borderBottom: 1,
            borderColor: 'divider',
            '& .MuiTabs-indicator': {
              display: 'none',
            },
            '& .MuiTab-root': {
              minHeight: 48,
              textTransform: 'none',
              fontSize: '14px',
              fontWeight: 400,
              color: theme.palette.text.secondary,
              position: 'relative',
              '&.Mui-selected': {
                color: theme.palette.primary.main,
                fontWeight: 500,
              },
              '&.Mui-selected::after': {
                content: '""',
                position: 'absolute',
                bottom: 0,
                left: '50%',
                transform: 'translateX(-50%)',
                width: '40px',
                height: '2px',
                backgroundColor: theme.palette.primary.main,
                zIndex: 1,
              },
            },
          }}
        >
          {SystemTabs.map(tab => (
            <Tab key={tab.id} label={tab.label} value={tab.id} />
          ))}
        </Tabs>
        {activeTab === 'user-management' && (
          <Box>
            <Member />
          </Box>
        )}
        {activeTab === 'model-config' && (
          <Box>
            <ModelConfig
              ref={modelConfigRef}
              onCloseModal={() => setOpen(false)}
              chatModelData={chatModelData}
              embeddingModelData={embeddingModelData}
              rerankModelData={rerankModelData}
              analysisModelData={analysisModelData}
              analysisVLModelData={analysisVLModelData}
              getModelList={getModelList}
            />
          </Box>
        )}
      </Modal>
    </>
  );
};
export default System;
