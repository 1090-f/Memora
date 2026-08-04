import { useEffect, useState, useMemo } from 'react';
import { CommonItem, StyledCommonWrapper } from '../../components/StyledCommon';
import { TextField } from '@mui/material';
import { Controller, useForm } from 'react-hook-form';
import DragList from '../../components/DragList';
import SortableItem from '../../components/SortableItem';
import Item from './Item';
import { Empty } from '@ctzhian/ui';
import type { ConfigProps } from '../type';
import { useAppSelector } from '@/store';
import { getApiV1NodeListGroupNav } from '@/request/Node';
import { convertToTree } from '@/utils/drag';
import AddNavContent from './AddNavContent';
import useDebounceAppPreviewData from '@/hooks/useDebounceAppPreviewData';
import { DEFAULT_DATA } from '../../../constants';
import { findConfigById, handleLandingConfigs } from '../../../utils';

const NavDocConfig = ({ setIsEdit, id }: ConfigProps) => {
  const { appPreviewData, kb_id } = useAppSelector(state => state.config);
  const debouncedDispatch = useDebounceAppPreviewData();
  const { control, setValue, watch, subscribe, reset } = useForm<
    typeof DEFAULT_DATA.nav_doc
  >({
    defaultValues: findConfigById(
      appPreviewData?.settings?.web_app_landing_configs || [],
      id,
    ),
  });

  const nodes = watch('nodes') || [];
  const [open, setOpen] = useState(false);

  const nodeRec = (ids: string[]) => {
    getApiV1NodeListGroupNav({ kb_id, nav_ids: ids, status: 'released' }).then(
      res => {
        setValue(
          'nodes',
          res.map(item => {
            const navTreeList = item.list ? convertToTree(item.list || []) : [];
            return {
              ...item,
              id: item.nav_id!,
              name: item.nav_name,
              list: navTreeList,
            };
          }),
        );
      },
    );
  };

  const handleListChange = (ids: string[]) => {
    setIsEdit(true);
    nodeRec(ids);
  };

  // 稳定的 SortableItemComponent 引用
  const ItemSortableComponent = useMemo(
    () => (props: any) => <SortableItem {...props} ItemComponent={Item} />,
    [],
  );

  useEffect(() => {
    reset(
      findConfigById(
        appPreviewData?.settings?.web_app_landing_configs || [],
        id,
      ),
      { keepDefaultValues: true },
    );
  }, [appPreviewData, id]);

  useEffect(() => {
    const callback = subscribe({
      formState: {
        values: true,
      },
      callback: ({ values }) => {
        const previewData = {
          ...appPreviewData,
          settings: {
            ...appPreviewData?.settings,
            web_app_landing_configs: handleLandingConfigs({
              id,
              config: appPreviewData?.settings?.web_app_landing_configs || [],
              values,
            }),
          },
        };
        setIsEdit(true);
        debouncedDispatch(previewData);
      },
    });
    return () => {
      callback();
    };
  }, [subscribe, id, appPreviewData]);

  return (
    <StyledCommonWrapper>
      {/* 标题配置 */}
      <CommonItem title='标题'>
        <Controller
          control={control}
          name='title'
          render={({ field }) => (
            <TextField label='文字' {...field} placeholder='请输入' />
          )}
        />
      </CommonItem>

      {/* 推荐目录列表 */}
      <CommonItem title='推荐目录' onAdd={() => setOpen(true)}>
        {nodes.length === 0 ? (
          <Empty />
        ) : (
          <DragList
            data={nodes}
            onChange={value => {
              setIsEdit(true);
              setValue('nodes', value);
            }}
            setIsEdit={setIsEdit}
            SortableItemComponent={ItemSortableComponent}
            ItemComponent={Item}
          />
        )}
      </CommonItem>

      {/* 添加目录弹窗：只选择导航目录名称 */}
      <AddNavContent
        open={open}
        selected={nodes.map(item => item.nav_id!)}
        onChange={handleListChange}
        onClose={() => setOpen(false)}
      />
    </StyledCommonWrapper>
  );
};

export default NavDocConfig;
