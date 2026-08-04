import { putApiV1App } from '@/request/App';

import { FormItem, SettingCardItem } from './Common';
import {
  DomainAppDetailResp,
  DomainConversationSetting,
} from '@/request/types';
import { PROFESSION_VERSION_PERMISSION } from '@/constant/version';
import {
  FormControlLabel,
  Radio,
  RadioGroup,
  TextField,
  Box,
} from '@mui/material';
import { message } from '@ctzhian/ui';
import { useEffect, useState } from 'react';
import VersionMask from '@/components/VersionMask';
import { Controller, useForm } from 'react-hook-form';
import { useAppSelector } from '@/store';

const CardQaCopyright = ({
  data,
  refresh,
}: {
  data: DomainAppDetailResp;
  refresh: (value: DomainConversationSetting) => void;
}) => {
  const [isEdit, setIsEdit] = useState<boolean>(false);
  const { kb_id } = useAppSelector(state => state.config);
  const {
    control,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors },
  } = useForm({
    defaultValues: {
      copyright_hide_enabled: false,
      copyright_info: '',
    },
  });

  const copyright_hide_enabled = watch('copyright_hide_enabled');

  const onSubmit = handleSubmit(value => {
    putApiV1App(
      { id: data.id! },
      { settings: { ...data.settings, conversation_setting: value }, kb_id },
    ).then(() => {
      refresh(value);
      message.success('保存成功');
      setIsEdit(false);
    });
  });

  useEffect(() => {
    setValue(
      'copyright_hide_enabled',
      data.settings?.conversation_setting?.copyright_hide_enabled ?? false,
    );
    setValue(
      'copyright_info',
      data.settings?.conversation_setting?.copyright_info ?? '',
    );
  }, [data]);

  return (
    <SettingCardItem
      title='智能问答版权信息'
      isEdit={isEdit}
      onSubmit={onSubmit}
    >
      <VersionMask permission={PROFESSION_VERSION_PERMISSION}>
        <FormItem
          label='版权信息'
          sx={{ alignItems: 'flex-start' }}
          labelSx={{ mt: 1 }}
        >
          <Controller
            control={control}
            name='copyright_hide_enabled'
            render={({ field }) => {
              return (
                <RadioGroup
                  row
                  {...field}
                  onChange={e => {
                    field.onChange(e.target.value === 'true');
                    setIsEdit(true);
                  }}
                >
                  <FormControlLabel
                    value={false}
                    control={<Radio size='small' />}
                    label={<Box sx={{ width: 100 }}>显示</Box>}
                  />
                  <FormControlLabel
                    value={true}
                    control={<Radio size='small' />}
                    label={<Box sx={{ width: 100 }}>隐藏</Box>}
                  />
                </RadioGroup>
              );
            }}
          />
        </FormItem>
        {!copyright_hide_enabled && (
          <FormItem
            label='版权文字'
            sx={{ alignItems: 'flex-start' }}
            labelSx={{ mt: 1 }}
          >
            <Controller
              control={control}
              name='copyright_info'
              render={({ field }) => (
                <TextField
                  fullWidth
                  {...field}
                  placeholder='本网站由 PandaWiki 提供技术支持'
                  error={!!errors.copyright_info}
                  helperText={errors.copyright_info?.message}
                  onChange={event => {
                    setIsEdit(true);
                    field.onChange(event);
                  }}
                />
              )}
            />
          </FormItem>
        )}
      </VersionMask>
    </SettingCardItem>
  );
};

export default CardQaCopyright;
