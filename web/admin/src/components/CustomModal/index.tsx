import { useEffect, useState, useRef } from 'react';
import { Button, Stack, Typography } from '@mui/material';
import { CusTabs, message, Modal } from '@ctzhian/ui';
import {
  getApiV1NodeRecommendNodes,
  getApiV1KnowledgeBaseDetail,
  getApiV1NodeListGroupNav,
} from '@/request';
import { convertToTree } from '@/utils/drag';
import {
  DomainAppDetailResp,
  DomainWebAppLandingConfigResp,
} from '@/request/types';
import { IconPCduan, IconYidongduan } from '@panda-wiki/icons';

import { getApiV1AppDetail, putApiV1App } from '@/request/App';
import { useAppSelector, useAppDispatch } from '@/store';
import { setAppPreviewData } from '@/store/slices/config';
import ComponentBar from './components/components/ComponentBar';
import ConfigBar from './components/config/ConfigBar';
import ShowContent from './components/ShowContent';
import {
  COMPONENTS_MAP,
  DEFAULT_DATA,
  TYPE_TO_CONFIG_LABEL,
} from './constants';
import { v4 as uuidv4 } from 'uuid';

type WebAppLandingConfigWithId = DomainWebAppLandingConfigResp & { id: string };

interface CustomModalProps {
  open: boolean;
  onCancel: () => void;
  refresh: (v: any) => void;
  disabledComponents?: string[];
  components?: string[] | null;
  title?: string;
}

export interface Component {
  id: string;
  name: string;
  title: string;
  component: React.FC<any>;
  config: React.FC<any>;
  fixed?: boolean;
  disabled?: boolean;
  hidden?: boolean;
}

const CustomModal = ({
  open,
  onCancel,
  refresh,
  title,
  disabledComponents,
  components: propsComponents,
}: CustomModalProps) => {
  const dispatch = useAppDispatch();
  const { kb_id } = useAppSelector(state => state.config);
  const [info, setInfo] = useState<DomainAppDetailResp>();
  const [renderMode, setRenderMode] = useState<'pc' | 'mobile'>('pc');
  const bannerRefId = useRef<string>(uuidv4());
  const [components, setComponents] = useState<Component[]>([
    { ...COMPONENTS_MAP.header, id: uuidv4() },
    { ...COMPONENTS_MAP.banner, id: bannerRefId.current },
    { ...COMPONENTS_MAP.footer, id: uuidv4() },
  ]);
  const [curComponent, setCurComponent] = useState<Component>(components[0]);
  const [isEdit, setIsEdit] = useState(false);
  const [scale, setScale] = useState(0.6);
  const [baseUrl, setBaseUrl] = useState('');
  const appPreviewData = useAppSelector(state => state.config.appPreviewData);

  const getInfo = async () => {
    const res = await getApiV1AppDetail({ kb_id: kb_id, type: '1' });
    const web_app_landing_configs = res.settings?.web_app_landing_configs || [];

    await Promise.all(
      web_app_landing_configs
        .map((item, index) => {
          if (item.node_ids && item.node_ids.length > 0) {
            return getApiV1NodeRecommendNodes({
              kb_id,
              node_ids: item.node_ids,
            }).then(res => {
              const label =
                TYPE_TO_CONFIG_LABEL[
                  item.type as keyof typeof TYPE_TO_CONFIG_LABEL
                ];
              (web_app_landing_configs[index] as any)[label] = {
                ...item[label],
                nodes: res,
              };
            });
          } else if (
            item.type === 'nav_doc' &&
            item.nav_doc_config?.nav_ids &&
            item.nav_doc_config.nav_ids.length > 0
          ) {
            return getApiV1NodeListGroupNav({
              kb_id,
              nav_ids: item.nav_doc_config.nav_ids,
            }).then(res => {
              const label =
                TYPE_TO_CONFIG_LABEL[
                  item.type as keyof typeof TYPE_TO_CONFIG_LABEL
                ];
              (web_app_landing_configs[index] as any)[label] = {
                ...item[label],
                nodes: res.map(item => {
                  const navTreeList = item.list
                    ? convertToTree(item.list || [])
                    : [];
                  return {
                    ...item,
                    id: item.nav_id,
                    name: item.nav_name,
                    list: navTreeList,
                  };
                }),
              };
            });
          }
        })
        .filter(Boolean),
    );
    setInfo(res);
    handleInitComponents(res);
  };
  const onSubmit = () => {
    if (!info || !appPreviewData) return;

    const submitWebAppLandingConfigs = components
      .map(item => {
        if (item.name === 'header' || item.name === 'footer') return null;
        const config = appPreviewData.settings?.web_app_landing_configs?.find(
          (con: any) => con.id === item.id,
        );

        const params: any = {
          type: config!.type,
          [TYPE_TO_CONFIG_LABEL[
            config!.type as keyof typeof TYPE_TO_CONFIG_LABEL
          ]]: {
            ...config,
          },
          node_ids: (config!.nodes?.map(node => node?.id) || []) as string[],
        };

        if (config!.type === 'nav_doc') {
          params.nav_doc_config.nav_ids = ((
            config!.nodes as Array<{ nav_id?: string }> | undefined
          )?.map(node => node?.nav_id) || []) as string[];
          delete params.node_ids;
        }

        return params;
      })
      .filter(Boolean);

    putApiV1App(
      { id: info.id! },
      {
        settings: {
          ...info.settings,
          ...appPreviewData.settings,
          web_app_landing_configs: submitWebAppLandingConfigs,
        },
        kb_id,
      },
    ).then(() => {
      refresh({
        ...appPreviewData.settings,
        web_app_landing_configs: submitWebAppLandingConfigs,
      });
      message.success('保存成功');
      setIsEdit(false);
    });
  };
  const zoomIn = () => {
    setScale(prev => Math.min(prev + 0.1, 2)); // 最大放大到200%
  };

  const zoomOut = () => {
    setScale(prev => Math.max(prev - 0.1, 0.5)); // 最小缩小到50%
  };

  const resetZoom = () => {
    setScale(1);
  };

  const handleInitComponents = (info: DomainAppDetailResp) => {
    const mergeInfo = { ...info };
    const web_app_landing_configs =
      info.settings?.web_app_landing_configs || [];
    if (web_app_landing_configs.length === 0) {
      mergeInfo.settings = {
        ...mergeInfo.settings,
        web_app_landing_configs: [
          {
            type: 'banner',
            id: bannerRefId.current,
            ...DEFAULT_DATA.banner,
          } as WebAppLandingConfigWithId,
        ],
      };
    } else {
      const newWebAppLandingConfigs = web_app_landing_configs.map(item => {
        return {
          id:
            item.type === 'banner'
              ? bannerRefId.current
              : (item as any).id || uuidv4(),
          type: item.type,
          ...(item as any)[
            TYPE_TO_CONFIG_LABEL[item.type as keyof typeof TYPE_TO_CONFIG_LABEL]
          ],
        };
      });

      mergeInfo.settings = {
        ...mergeInfo.settings,
        web_app_landing_configs: newWebAppLandingConfigs,
      };

      setComponents(pre => {
        let customComponents = newWebAppLandingConfigs.map(item => {
          return {
            ...COMPONENTS_MAP[item.type as keyof typeof COMPONENTS_MAP],
            id: item.id,
          };
        });
        // @ts-expect-error ignore
        customComponents = [pre[0], ...customComponents, pre[pre.length - 1]];

        if (propsComponents) {
          customComponents = customComponents.map(item => ({
            ...item,
            hidden: !propsComponents?.includes(item.name),
          }));
        }
        if ((disabledComponents || []).length > 0) {
          customComponents = customComponents.map(item => ({
            ...item,
            disabled: disabledComponents?.includes(item.name) || false,
          }));
        }
        setCurComponent(
          customComponents.find(item => !item.disabled && !item.hidden) ||
            customComponents[0],
        );
        return customComponents;
      });
    }
    dispatch(setAppPreviewData(mergeInfo));
  };

  useEffect(() => {
    if (open && kb_id) {
      getApiV1KnowledgeBaseDetail({ id: kb_id }).then(res => {
        if (res.access_settings?.base_url) {
          setBaseUrl(res!.access_settings!.base_url!);
        } else {
          let defaultUrl: string = '';
          const host = res.access_settings?.hosts?.[0] || '';
          if (!host) return;

          if (
            res.access_settings?.ssl_ports &&
            res.access_settings?.ssl_ports.length > 0
          ) {
            defaultUrl = res.access_settings.ssl_ports.includes(443)
              ? `https://${host}`
              : `https://${host}:${res.access_settings.ssl_ports[0]}`;
          } else if (
            res.access_settings?.ports &&
            res.access_settings?.ports.length > 0
          ) {
            defaultUrl = res.access_settings.ports.includes(80)
              ? `http://${host}`
              : `http://${host}:${res.access_settings.ports[0]}`;
          }
          setBaseUrl(defaultUrl);
        }
      });
      getInfo();
    }
  }, [kb_id, open]);

  useEffect(() => {
    if (!open) {
      setTimeout(() => {
        setScale(0.6);
        setIsEdit(false);
        setComponents([
          { ...COMPONENTS_MAP.header, id: uuidv4() },
          { ...COMPONENTS_MAP.banner, id: bannerRefId.current },
          { ...COMPONENTS_MAP.footer, id: uuidv4() },
        ]);
      }, 300);
    }
  }, [open]);

  return (
    <>
      {open && (
        <Modal
          open={open}
          onCancel={onCancel}
          width={'95%'}
          footer={null}
          style={{}}
          sx={{
            maxWidth: 'none',
            '& .MuiDialog-paper': {
              maxWidth: 'none',
              bgcolor: '#FFFFFF',
              padding: '0px',
              margin: 0,
              maxHeight: '90%',
              height: '90%',
            },
            '& .MuiDialogContent-root': {
              maxWidth: 'none',
              padding: '0px',
              height: '100%',
              display: 'flex',
              overflow: 'hidden',
            },
            '& .MuiDialogTitle-root': {
              padding: '0px',
            },
          }}
          title={
            <Stack
              direction='row'
              gap={2}
              sx={{
                width: '100%',
                bgcolor: '#FFFFFF',
                height: '64px',
                borderBottom: '1px solid #ECEEF1',
                alignItems: 'center',
                paddingLeft: '20px',
              }}
            >
              <Typography
                sx={{
                  fontSize: '16px',
                  lineHeight: '24px',
                  fontWeight: 600,
                }}
              >
                {title || '自定义欢迎页'}
              </Typography>
              <Button
                variant='contained'
                size='small'
                disabled={!isEdit}
                onClick={onSubmit}
              >
                保存
              </Button>

              {appPreviewData && (
                <Stack
                  direction='row'
                  gap={1}
                  alignItems='center'
                  sx={{
                    marginLeft: 'auto',
                    marginRight: '80px',
                    marginTop: '6px',
                  }}
                >
                  <CusTabs
                    list={[
                      {
                        label: <IconPCduan sx={{ fontSize: 14 }} />,
                        value: 'pc',
                      },
                      {
                        label: <IconYidongduan sx={{ fontSize: 14 }} />,
                        value: 'mobile',
                      },
                    ]}
                    value={renderMode}
                    onChange={value => {
                      if (value === 'pc' || value === 'mobile') {
                        setRenderMode(value);
                      }
                    }}
                  ></CusTabs>
                  <Stack direction='row' gap={1}>
                    <Button
                      variant='outlined'
                      size='small'
                      onClick={zoomOut}
                      sx={{ minWidth: '30px', padding: '4px' }}
                    >
                      -
                    </Button>
                    <Button
                      variant='outlined'
                      size='small'
                      onClick={resetZoom}
                      sx={{ minWidth: '60px', padding: '4px' }}
                    >
                      {Math.round(scale * 100)}%
                    </Button>
                    <Button
                      variant='outlined'
                      size='small'
                      onClick={zoomIn}
                      sx={{ minWidth: '30px', padding: '4px' }}
                    >
                      +
                    </Button>
                  </Stack>
                </Stack>
              )}
            </Stack>
          }
        >
          <Stack
            direction={'row'}
            gap={2}
            sx={{
              width: '100%',
              minWidth: 0,
              bgcolor: 'background.paper2',
              minHeight: 0,
              height: '100%',
              ml: '20px',
            }}
          >
            {!propsComponents && (
              <ComponentBar
                components={components}
                setComponents={setComponents}
                curComponent={curComponent}
                setCurComponent={setCurComponent}
                setIsEdit={setIsEdit}
                allowAdd={!propsComponents}
              />
            )}

            {appPreviewData ? (
              <ShowContent
                curComponent={curComponent}
                components={components}
                setComponents={setComponents}
                setIsEdit={setIsEdit}
                setCurComponent={setCurComponent}
                renderMode={renderMode}
                scale={scale}
                baseUrl={baseUrl}
              />
            ) : (
              <Stack sx={{ width: '100%' }}>loading...</Stack>
            )}

            <ConfigBar
              curComponent={curComponent}
              components={components}
              setIsEdit={setIsEdit}
              data={info as any}
              isEdit={isEdit}
            ></ConfigBar>
          </Stack>
        </Modal>
      )}
    </>
  );
};
export default CustomModal;
