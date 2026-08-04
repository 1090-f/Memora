import { getApiProV1Prompt, putApiProV1Prompt } from '@/request/pro/Prompt';
import { DomainKnowledgeBaseDetail } from '@/request/types';
import { PROFESSION_VERSION_PERMISSION } from '@/constant/version';
import { useAppSelector } from '@/store';
import { message, Modal } from '@ctzhian/ui';
import VersionMask from '@/components/VersionMask';
import {
  Box,
  FormControlLabel,
  RadioGroup,
  Radio,
  TextField,
  styled,
} from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { FormItem, SettingCardItem } from './Common';
import { DomainUpdatePromptReq } from '@/request/pro/types';

interface CardAIProps {
  kb: DomainKnowledgeBaseDetail;
}

const StyledRadioLabel = styled(Box)(({ theme }) => ({
  width: 100,
}));

const CardAI = ({ kb }: CardAIProps) => {
  const [isEdit, setIsEdit] = useState(false);
  const { license } = useAppSelector(state => state.config);

  const { control, handleSubmit, setValue, getValues, watch } = useForm({
    defaultValues: {
      interval: 0,
      content: '',
      summary_content: '',
      enable_preset: false,
      enable_preset_auto_language: true,
      enable_preset_general_info: true,
      enable_preset_reference: true,
    },
  });

  const enable_preset = watch('enable_preset');

  const onSubmit = handleSubmit(async data => {
    await putApiProV1Prompt({
      kb_id: kb.id!,
      content: data.content,
      summary_content: data.summary_content,
      enable_preset: data.enable_preset,
      enable_preset_auto_language: data.enable_preset_auto_language,
      enable_preset_general_info: data.enable_preset_general_info,
      enable_preset_reference: data.enable_preset_reference,
    });

    message.success('保存成功');
    setIsEdit(false);
  });

  const isPro = useMemo(() => {
    return PROFESSION_VERSION_PERMISSION.includes(license.edition!);
  }, [license]);

  useEffect(() => {
    if (!kb.id || !PROFESSION_VERSION_PERMISSION.includes(license.edition!))
      return;
    getApiProV1Prompt({ kb_id: kb.id! }).then(res => {
      setValue('content', res.content || '');
      setValue('summary_content', res.summary_content || '');
      setValue('enable_preset', res.enable_preset ?? false);
      setValue(
        'enable_preset_auto_language',
        res.enable_preset_auto_language ?? true,
      );
      setValue(
        'enable_preset_general_info',
        res.enable_preset_general_info ?? true,
      );
      setValue('enable_preset_reference', res.enable_preset_reference ?? true);
    });
  }, [kb, isPro]);

  const onResetPrompt = (type: 'content' | 'summary_content' = 'content') => {
    Modal.confirm({
      title: '提示',
      content: `确定要重置为默认${type === 'content' ? '智能问答' : '智能摘要'}提示词吗？`,
      onOk: () => {
        let params: DomainUpdatePromptReq = {
          kb_id: kb.id!,
          content: '',
          summary_content: getValues('summary_content'),
          enable_preset: getValues('enable_preset'),
          enable_preset_auto_language: getValues('enable_preset_auto_language'),
          enable_preset_general_info: getValues('enable_preset_general_info'),
          enable_preset_reference: getValues('enable_preset_reference'),
        };
        if (type === 'summary_content') {
          params = {
            kb_id: kb.id!,
            summary_content: '',
            content: getValues('content'),
            enable_preset: getValues('enable_preset'),
            enable_preset_auto_language: getValues(
              'enable_preset_auto_language',
            ),
            enable_preset_general_info: getValues('enable_preset_general_info'),
            enable_preset_reference: getValues('enable_preset_reference'),
          };
        }
        putApiProV1Prompt(params).then(() => {
          getApiProV1Prompt({ kb_id: kb.id! }).then(res => {
            setValue(type, res[type] || '');
            message.success('重置成功');
          });
        });
      },
    });
  };

  return (
    <Box
      sx={{
        width: 1000,
        margin: 'auto',
        pb: 4,
      }}
    >
      <SettingCardItem title='智能问答' isEdit={isEdit} onSubmit={onSubmit}>
        <VersionMask permission={PROFESSION_VERSION_PERMISSION}>
          <FormItem label='智能问答提示词'>
            <Controller
              control={control}
              name='enable_preset'
              render={({ field }) => (
                <RadioGroup
                  row
                  {...field}
                  onChange={e => {
                    setIsEdit(true);
                    field.onChange(e.target.value === 'true');
                  }}
                >
                  <FormControlLabel
                    value={false}
                    control={<Radio size='small' />}
                    label={<StyledRadioLabel>自定义</StyledRadioLabel>}
                  />
                  <FormControlLabel
                    value={true}
                    control={<Radio size='small' />}
                    label={<StyledRadioLabel>通用配置</StyledRadioLabel>}
                  />
                </RadioGroup>
              )}
            />
          </FormItem>

          {!enable_preset ? (
            <FormItem
              vertical
              extra={
                <Box
                  sx={{
                    fontSize: 12,
                    color: 'primary.main',
                    display: 'block',
                    cursor: 'pointer',
                  }}
                  onClick={() => onResetPrompt('content')}
                >
                  重置为默认提示词
                </Box>
              }
              label=''
            >
              <Controller
                control={control}
                name='content'
                render={({ field }) => (
                  <TextField
                    {...field}
                    fullWidth
                    disabled={!isPro}
                    multiline
                    rows={20}
                    placeholder='智能问答提示词'
                    onChange={e => {
                      field.onChange(e.target.value);
                      setIsEdit(true);
                    }}
                  />
                )}
              />
            </FormItem>
          ) : (
            <>
              <FormItem label='自动匹配语言回复'>
                <Controller
                  control={control}
                  name='enable_preset_auto_language'
                  render={({ field }) => (
                    <RadioGroup
                      row
                      {...field}
                      onChange={e => {
                        setIsEdit(true);
                        field.onChange(e.target.value === 'true');
                      }}
                    >
                      <FormControlLabel
                        value={true}
                        control={<Radio size='small' />}
                        label={<StyledRadioLabel>启用</StyledRadioLabel>}
                      />
                      <FormControlLabel
                        value={false}
                        control={<Radio size='small' />}
                        label={<StyledRadioLabel>禁用</StyledRadioLabel>}
                      />
                    </RadioGroup>
                  )}
                />
              </FormItem>
              <FormItem label='结合通用知识补充回答'>
                <Controller
                  control={control}
                  name='enable_preset_general_info'
                  render={({ field }) => (
                    <RadioGroup
                      row
                      {...field}
                      onChange={e => {
                        setIsEdit(true);
                        field.onChange(e.target.value === 'true');
                      }}
                    >
                      <FormControlLabel
                        value={true}
                        control={<Radio size='small' />}
                        label={<StyledRadioLabel>启用</StyledRadioLabel>}
                      />
                      <FormControlLabel
                        value={false}
                        control={<Radio size='small' />}
                        label={<StyledRadioLabel>禁用</StyledRadioLabel>}
                      />
                    </RadioGroup>
                  )}
                />
              </FormItem>
              <FormItem label='回答中显示引用来源'>
                <Controller
                  control={control}
                  name='enable_preset_reference'
                  render={({ field }) => (
                    <RadioGroup
                      row
                      {...field}
                      onChange={e => {
                        setIsEdit(true);
                        field.onChange(e.target.value === 'true');
                      }}
                    >
                      <FormControlLabel
                        value={true}
                        control={<Radio size='small' />}
                        label={<StyledRadioLabel>启用</StyledRadioLabel>}
                      />
                      <FormControlLabel
                        value={false}
                        control={<Radio size='small' />}
                        label={<StyledRadioLabel>禁用</StyledRadioLabel>}
                      />
                    </RadioGroup>
                  )}
                />
              </FormItem>
            </>
          )}
          <FormItem
            vertical
            extra={
              <Box
                sx={{
                  fontSize: 12,
                  color: 'primary.main',
                  display: 'block',
                  cursor: 'pointer',
                }}
                onClick={() => onResetPrompt('summary_content')}
              >
                重置为默认提示词
              </Box>
            }
            label='智能摘要提示词'
          >
            <Controller
              control={control}
              name='summary_content'
              render={({ field }) => (
                <TextField
                  {...field}
                  fullWidth
                  disabled={!isPro}
                  multiline
                  rows={5}
                  placeholder='智能摘要提示词'
                  onChange={e => {
                    field.onChange(e.target.value);
                    setIsEdit(true);
                  }}
                />
              )}
            />
          </FormItem>
        </VersionMask>

        {/* <FormItem vertical label='连续提问时间间隔（敬请期待）'>
          <Controller
            control={control}
            name='interval'
            render={({ field }) => (
              <Slider
                {...field}
                disabled
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
                      transform:
                        'translate(50%, -100%) rotate(-45deg) scale(1)',
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
        </FormItem> */}
      </SettingCardItem>
    </Box>
  );
};

export default CardAI;
