import { CatalogSetting } from '@/api/type';
import { putApiV1App } from '@/request/App';
import { DomainAppDetailResp } from '@/request/types';
import {
  Box,
  FormControlLabel,
  Radio,
  RadioGroup,
  Slider,
} from '@mui/material';
import { message } from '@ctzhian/ui';
import { useEffect, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { FormItem, SettingCardItem } from './Common';
import { useAppSelector } from '@/store';

interface CardCatalogProps {
  id: string;
  data: DomainAppDetailResp;
  refresh: (value: CatalogSetting) => void;
}

const CardCatalog = ({ id, data, refresh }: CardCatalogProps) => {
  const [isEdit, setIsEdit] = useState(false);
  const { kb_id } = useAppSelector(state => state.config);
  const { control, handleSubmit, setValue } = useForm<CatalogSetting>({
    defaultValues: {
      catalog_visible: 1,
      catalog_folder: 1,
      catalog_width: 260,
    },
  });

  const onSubmit = handleSubmit(value => {
    putApiV1App(
      { id },
      { settings: { ...data.settings, catalog_settings: value }, kb_id },
    ).then(() => {
      refresh(value);
      message.success('保存成功');
      setIsEdit(false);
    });
  });

  useEffect(() => {
    setValue(
      'catalog_visible',
      (data.settings?.catalog_settings?.catalog_visible || 1) as 1 | 2,
    );
    setValue(
      'catalog_folder',
      (data.settings?.catalog_settings?.catalog_folder || 1) as 1 | 2,
    );
    setValue(
      'catalog_width',
      data.settings?.catalog_settings?.catalog_width ?? 260,
    );
  }, [data]);

  return (
    <SettingCardItem title='左侧目录导航' isEdit={isEdit} onSubmit={onSubmit}>
      <FormItem label='左侧目录导航'>
        <Controller
          control={control}
          name='catalog_visible'
          render={({ field }) => (
            <RadioGroup
              row
              {...field}
              onChange={e => {
                field.onChange(+e.target.value as 1 | 2);
                setIsEdit(true);
              }}
            >
              <FormControlLabel
                value={1}
                control={<Radio size='small' />}
                label={<Box sx={{ width: 100 }}>默认显示</Box>}
              />
              <FormControlLabel
                value={2}
                control={<Radio size='small' />}
                label={<Box sx={{ width: 100 }}>默认隐藏</Box>}
              />
            </RadioGroup>
          )}
        />
      </FormItem>

      <FormItem label='文件夹'>
        <Controller
          control={control}
          name='catalog_folder'
          render={({ field }) => (
            <RadioGroup
              row
              {...field}
              onChange={e => {
                field.onChange(+e.target.value as 1 | 2);
                setIsEdit(true);
              }}
            >
              <FormControlLabel
                value={1}
                control={<Radio size='small' />}
                label={<Box sx={{ width: 100 }}>默认展开</Box>}
              />
              <FormControlLabel
                value={2}
                control={<Radio size='small' />}
                label={<Box sx={{ width: 100 }}>默认折叠</Box>}
              />
            </RadioGroup>
          )}
        />
      </FormItem>

      <FormItem label='导航宽度'>
        <Controller
          control={control}
          name='catalog_width'
          render={({ field }) => (
            <Slider
              {...field}
              valueLabelDisplay='auto'
              min={200}
              max={300}
              step={5}
              sx={{
                width: 432,
                '& .MuiSlider-thumb': {
                  width: 16,
                  height: 16,
                  borderRadius: '50%',
                  backgroundColor: '#fff',
                  border: '2px solid currentColor',
                  '&:focus, &:hover, &.Mui-active, &.Mui-focusVisible': {
                    boxShadow: 'inherit',
                  },
                  '&::before': {
                    display: 'none',
                  },
                },
                '& .MuiSlider-track': {
                  bgcolor: 'primary.main',
                },
                '& .MuiSlider-rail': {
                  bgcolor: 'text.disabled',
                },
                '& .MuiSlider-valueLabel': {
                  lineHeight: 1.2,
                  fontSize: 12,
                  fontWeight: 'bold',
                  background: 'unset',
                  p: 0,
                  width: 24,
                  height: 24,
                  borderRadius: '50% 50% 50% 0',
                  bgcolor: 'primary.main',
                  transformOrigin: 'bottom left',
                  transform: 'translate(50%, -100%) rotate(-45deg) scale(0)',
                  '&::before': { display: 'none' },
                  '&.MuiSlider-valueLabelOpen': {
                    transform: 'translate(50%, -100%) rotate(-45deg) scale(1)',
                  },
                  '& > *': {
                    transform: 'rotate(45deg)',
                  },
                },
              }}
              onChange={(e, value) => {
                field.onChange(+value);
                setIsEdit(true);
              }}
            />
          )}
        />
      </FormItem>
    </SettingCardItem>
  );
};

export default CardCatalog;
