import React, { useEffect, useRef, useMemo } from 'react';
import { TextField } from '@mui/material';
import { CommonItem, StyledCommonWrapper } from '../../components/StyledCommon';
import type { ConfigProps } from '../type';
import { useForm, Controller } from 'react-hook-form';
import { useAppSelector } from '@/store';
import DragList from '../../components/DragList';
import SortableItem from '../../components/SortableItem';
import Item from './Item';
import HotSearchItem from './HotSearchItem';
import UploadFile from '@/components/UploadFile';
import { DEFAULT_DATA } from '../../../constants';
import useDebounceAppPreviewData from '@/hooks/useDebounceAppPreviewData';
import { handleLandingConfigs, findConfigById } from '../../../utils';
import { Empty } from '@ctzhian/ui';

const Config: React.FC<ConfigProps> = ({ setIsEdit, id }) => {
  const { appPreviewData } = useAppSelector(state => state.config);
  const { control, watch, setValue, subscribe } = useForm<
    typeof DEFAULT_DATA.banner
  >({
    defaultValues: findConfigById(
      appPreviewData?.settings?.web_app_landing_configs || [],
      id,
    ),
  });

  const debouncedDispatch = useDebounceAppPreviewData();
  const btns = watch('btns') || [];
  const hotSearch = watch('hot_search') || [];

  // 使用 ref 来维护稳定的 ID 映射
  const idMapRef = useRef<Map<number, string>>(new Map());

  // 将string[]转换为对象数组用于显示，保持 ID 稳定
  const hotSearchList = Array.isArray(hotSearch)
    ? hotSearch.map((text, index) => {
        // 如果该索引没有 ID，生成一个新的
        if (!idMapRef.current.has(index)) {
          idMapRef.current.set(
            index,
            `${Date.now()}-${index}-${Math.random()}`,
          );
        }
        return {
          id: idMapRef.current.get(index)!,
          text: String(text),
        };
      })
    : [];

  // 清理不再使用的 ID，并确保所有索引都有 ID
  useEffect(() => {
    const currentIndexes = new Set(hotSearch.map((_, index) => index));

    // 清理不存在的索引
    const keysToDelete: number[] = [];
    idMapRef.current.forEach((_, key) => {
      if (!currentIndexes.has(key)) {
        keysToDelete.push(key);
      }
    });
    keysToDelete.forEach(key => idMapRef.current.delete(key));

    // 确保每个索引都有 ID
    hotSearch.forEach((_, index) => {
      if (!idMapRef.current.has(index)) {
        idMapRef.current.set(index, `${Date.now()}-${index}-${Math.random()}`);
      }
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hotSearch.length]);

  const handleAddButton = () => {
    const nextId = `${Date.now()}`;
    setValue('btns', [
      ...(btns || []),
      { id: nextId, text: '', type: 'contained', href: '' },
    ]);
  };

  const handleAddHotSearch = () => {
    const newIndex = hotSearch.length;
    const nextId = `${Date.now()}-${newIndex}-${Math.random()}`;
    idMapRef.current.set(newIndex, nextId);
    // 转换回string[]格式
    setValue('hot_search', [...hotSearch, '']);
    setIsEdit(true);
  };

  const handleHotSearchChange = (newList: { id: string; text: string }[]) => {
    // 重建 ID 映射关系
    const newIdMap = new Map<number, string>();
    newList.forEach((item, index) => {
      newIdMap.set(index, item.id);
    });
    idMapRef.current = newIdMap;

    // 转换回string[]格式
    setValue(
      'hot_search',
      newList.map(item => item.text),
    );
    setIsEdit(true);
  };

  // 稳定的 SortableItemComponent 引用
  const HotSearchSortableItem = useMemo(
    () => (props: any) => (
      <SortableItem {...props} ItemComponent={HotSearchItem} />
    ),
    [],
  );

  const ButtonSortableItem = useMemo(
    () => (props: any) => <SortableItem {...props} ItemComponent={Item} />,
    [],
  );

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
              values: {
                ...values,
              },
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [subscribe, appPreviewData, id]);

  return (
    <StyledCommonWrapper>
      <CommonItem title='主标题'>
        <Controller
          control={control}
          name='title'
          render={({ field }) => (
            <TextField
              label='文字'
              slotProps={{
                inputLabel: {
                  shrink: true,
                },
              }}
              fullWidth
              {...field}
              placeholder='请输入'
            />
          )}
        />
      </CommonItem>
      <CommonItem title='副标题'>
        <Controller
          control={control}
          name='subtitle'
          render={({ field }) => (
            <TextField
              label='文字'
              slotProps={{
                inputLabel: {
                  shrink: true,
                },
              }}
              multiline
              minRows={2}
              fullWidth
              {...field}
              placeholder='请输入'
            />
          )}
        />
      </CommonItem>
      <CommonItem title='背景图' desc='(推荐 1920*720)'>
        <Controller
          control={control}
          name='bg_url'
          render={({ field }) => (
            <UploadFile
              name='bg_url'
              id='bannerconfig_bgUrl'
              type='url'
              disabled={false}
              accept='image/*'
              width={354}
              height={129}
              value={field.value || ''}
              onChange={(url: string) => {
                field.onChange(url);
                setIsEdit(true);
              }}
            />
          )}
        />
      </CommonItem>
      <CommonItem title='搜索框'>
        <Controller
          control={control}
          name='placeholder'
          render={({ field }) => <TextField {...field} placeholder='请输入' />}
        />
      </CommonItem>
      <CommonItem title='热门搜索' onAdd={handleAddHotSearch}>
        {hotSearchList.length === 0 ? (
          <Empty />
        ) : (
          <DragList
            data={hotSearchList}
            onChange={handleHotSearchChange}
            setIsEdit={setIsEdit}
            SortableItemComponent={HotSearchSortableItem}
            ItemComponent={HotSearchItem}
          />
        )}
      </CommonItem>
      <CommonItem title='主按钮' onAdd={handleAddButton}>
        <DragList
          data={btns as Required<(typeof btns)[0]>[]}
          onChange={btns => {
            setValue('btns', btns);
            setIsEdit(true);
          }}
          setIsEdit={setIsEdit}
          SortableItemComponent={ButtonSortableItem}
          ItemComponent={Item}
        />
      </CommonItem>
    </StyledCommonWrapper>
  );
};

export default Config;
