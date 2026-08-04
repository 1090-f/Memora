import React, { useEffect, useMemo } from 'react';
import { CommonItem, StyledCommonWrapper } from '../../components/StyledCommon';
import { TextField } from '@mui/material';
import { Controller, useForm } from 'react-hook-form';
import DragList from '../../components/DragList';
import SortableItem from '../../components/SortableItem';
import Item from './Item';
import type { ConfigProps } from '../type';
import { useAppSelector } from '@/store';
import useDebounceAppPreviewData from '@/hooks/useDebounceAppPreviewData';
import { Empty } from '@ctzhian/ui';
import { DEFAULT_DATA } from '../../../constants';
import { findConfigById, handleLandingConfigs } from '../../../utils';

const MetricsConfig = ({ setIsEdit, id }: ConfigProps) => {
  const { appPreviewData } = useAppSelector(state => state.config);
  const debouncedDispatch = useDebounceAppPreviewData();
  const { control, setValue, watch, reset, subscribe } = useForm<
    typeof DEFAULT_DATA.metrics
  >({
    defaultValues: findConfigById(
      appPreviewData?.settings?.web_app_landing_configs || [],
      id,
    ),
  });

  const list = watch('list') || [];

  const handleAddQuestion = () => {
    const nextId = `${Date.now()}`;
    setValue('list', [...list, { id: nextId, name: '', number: '' }]);
  };

  const handleListChange = (newList: (typeof DEFAULT_DATA.metrics)['list']) => {
    setValue('list', newList);
    setIsEdit(true);
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
  }, [id, appPreviewData]);

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
      <CommonItem title='标题'>
        <Controller
          control={control}
          name='title'
          render={({ field }) => (
            <TextField label='文字' {...field} placeholder='请输入' />
          )}
        />
      </CommonItem>

      <CommonItem title='指标列表' onAdd={handleAddQuestion}>
        {list.length === 0 ? (
          <Empty />
        ) : (
          <DragList
            data={list}
            onChange={handleListChange}
            setIsEdit={setIsEdit}
            SortableItemComponent={ItemSortableComponent}
            ItemComponent={Item}
          />
        )}
      </CommonItem>
    </StyledCommonWrapper>
  );
};

export default MetricsConfig;
