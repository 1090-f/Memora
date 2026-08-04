import React, { useEffect } from 'react';
import { CommonItem, StyledCommonWrapper } from '../../components/StyledCommon';
import { Stack, TextField } from '@mui/material';
import { Controller, useForm } from 'react-hook-form';
import type { ConfigProps } from '../type';
import { useAppSelector } from '@/store';
import useDebounceAppPreviewData from '@/hooks/useDebounceAppPreviewData';
import { DEFAULT_DATA } from '../../../constants';
import { findConfigById, handleLandingConfigs } from '../../../utils';
import UploadFile from '@/components/UploadFile';

const Config = ({ setIsEdit, id }: ConfigProps) => {
  const { appPreviewData } = useAppSelector(state => state.config);
  const debouncedDispatch = useDebounceAppPreviewData();
  const { control, setValue, watch, subscribe, reset } = useForm<
    typeof DEFAULT_DATA.img_text
  >({
    defaultValues: findConfigById(
      appPreviewData?.settings?.web_app_landing_configs || [],
      id,
    ),
  });

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
      <CommonItem title='标题'>
        <Controller
          control={control}
          name='title'
          render={({ field }) => (
            <TextField label='文字' {...field} placeholder='请输入' />
          )}
        />
      </CommonItem>
      <CommonItem title='图片' desc='(推荐 350*350，1 : 1 )'>
        <Controller
          control={control}
          name='item.url'
          render={({ field }) => (
            <UploadFile
              name='item.url'
              id={`${id}_icon`}
              type='url'
              disabled={false}
              accept='image/*'
              width={110}
              height={110}
              value={field.value}
              onChange={(url: string) => {
                field.onChange(url);
                setIsEdit(true);
              }}
            />
          )}
        />
      </CommonItem>
      <CommonItem title='内容'>
        <Controller
          control={control}
          name='item.name'
          render={({ field }) => (
            <TextField
              label='标题'
              {...field}
              placeholder='请输入'
              onChange={e => {
                setIsEdit(true);
                field.onChange(e.target.value);
              }}
            />
          )}
        />
        <Controller
          control={control}
          name='item.desc'
          render={({ field }) => (
            <TextField
              label='描述'
              {...field}
              placeholder='请输入'
              onChange={e => {
                setIsEdit(true);
                field.onChange(e.target.value);
              }}
            />
          )}
        />
      </CommonItem>
    </StyledCommonWrapper>
  );
};

export default Config;
