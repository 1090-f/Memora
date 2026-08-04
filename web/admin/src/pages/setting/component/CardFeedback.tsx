import {
  DomainAppDetailResp,
  DomainKnowledgeBaseDetail,
} from '@/request/types';
import { useAppSelector } from '@/store';
import { PROFESSION_VERSION_PERMISSION } from '@/constant/version';
import {
  Box,
  Chip,
  FormControlLabel,
  Radio,
  RadioGroup,
  styled,
  TextField,
} from '@mui/material';

import { getApiV1AppDetail, putApiV1App } from '@/request/App';
import { message } from '@ctzhian/ui';
import Autocomplete from '@mui/material/Autocomplete';
import { useEffect, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { FormItem, SettingCardItem } from './Common';

interface CardCommentProps {
  kb: DomainKnowledgeBaseDetail;
}

const StyledRadioLabel = styled(Box)(({ theme }) => ({
  width: 100,
}));

const DocumentComments = ({
  data,
  refresh,
}: {
  data: DomainAppDetailResp;
  refresh: () => void;
}) => {
  const { kb_id } = useAppSelector(state => state.config);
  const [isEdit, setIsEdit] = useState(false);
  const { control, handleSubmit, setValue } = useForm({
    defaultValues: {
      is_open: 0,
      moderation_enable: 0,
    },
  });

  useEffect(() => {
    // @ts-expect-error 忽略类型错误
    setValue('is_open', +data?.settings?.web_app_comment_settings?.is_enable);

    setValue(
      'moderation_enable',
      // @ts-expect-error 忽略类型错误
      +data?.settings?.web_app_comment_settings?.moderation_enable,
    );
  }, [data]);

  const onSubmit = handleSubmit(formData => {
    putApiV1App(
      { id: data.id! },
      {
        kb_id,
        settings: {
          ...data.settings,
          web_app_comment_settings: {
            ...data.settings?.web_app_comment_settings,
            is_enable: Boolean(formData.is_open),
            moderation_enable: Boolean(formData.moderation_enable),
          },
        },
      },
    ).then(() => {
      message.success('保存成功');
      setIsEdit(false);
      refresh();
    });
  });
  return (
    <SettingCardItem title='文档评论' isEdit={isEdit} onSubmit={onSubmit}>
      <FormItem label='文档评论'>
        <Controller
          control={control}
          name='is_open'
          render={({ field }) => (
            <RadioGroup
              row
              {...field}
              onChange={e => {
                setIsEdit(true);
                field.onChange(+e.target.value as 1 | 0);
              }}
            >
              <FormControlLabel
                value={1}
                control={<Radio size='small' />}
                label={<StyledRadioLabel>启用</StyledRadioLabel>}
              />
              <FormControlLabel
                value={0}
                control={<Radio size='small' />}
                label={<StyledRadioLabel>禁用</StyledRadioLabel>}
              />
            </RadioGroup>
          )}
        />
      </FormItem>
      <FormItem label='评论审核' permission={PROFESSION_VERSION_PERMISSION}>
        <Controller
          control={control}
          name='moderation_enable'
          render={({ field }) => (
            <RadioGroup
              row
              {...field}
              onChange={e => {
                setIsEdit(true);
                field.onChange(+e.target.value as 1 | 0);
              }}
            >
              <FormControlLabel
                value={1}
                control={<Radio size='small' />}
                label={<StyledRadioLabel>启用</StyledRadioLabel>}
              />
              <FormControlLabel
                value={0}
                control={<Radio size='small' />}
                label={<StyledRadioLabel>禁用</StyledRadioLabel>}
              />
            </RadioGroup>
          )}
        />
      </FormItem>
    </SettingCardItem>
  );
};

const AI_FEEDBACK_OPTIONS = ['内容不准确', '答非所问', '其他'];

const AIQuestion = ({
  data,
  refresh,
}: {
  data: DomainAppDetailResp;
  refresh: () => void;
}) => {
  const [isEdit, setIsEdit] = useState(false);
  const { kb_id } = useAppSelector(state => state.config);
  const { control, handleSubmit, setValue } = useForm({
    defaultValues: {
      is_enabled: true,
      ai_feedback_type: [],
      disclaimer: '',
    },
  });
  const [inputValue, setInputValue] = useState('');

  const onSubmit = handleSubmit(formData => {
    putApiV1App(
      { id: data.id! },
      {
        kb_id,
        settings: {
          ...data.settings,
          ai_feedback_settings: {
            is_enabled: formData.is_enabled,
            ai_feedback_type: formData.ai_feedback_type,
          },
          disclaimer_settings: {
            content: formData.disclaimer,
          },
        },
      },
    ).then(() => {
      message.success('保存成功');
      setIsEdit(false);
      refresh();
    });
  });

  useEffect(() => {
    setValue(
      'is_enabled',
      data.settings?.ai_feedback_settings?.is_enabled ?? true,
    );

    setValue(
      'ai_feedback_type',
      // @ts-expect-error 忽略类型错误
      data.settings?.ai_feedback_settings?.ai_feedback_type || [],
    );
    setValue(
      'disclaimer',
      data.settings?.disclaimer_settings?.content as string,
    );
  }, [data]);

  return (
    <SettingCardItem title='AI 问答评价' isEdit={isEdit} onSubmit={onSubmit}>
      <FormItem label='AI 问答评价'>
        <Controller
          control={control}
          name='ai_feedback_type'
          render={({ field }) => (
            <Autocomplete
              {...field}
              multiple
              freeSolo
              fullWidth
              options={AI_FEEDBACK_OPTIONS}
              inputValue={inputValue}
              onInputChange={(_, newInputValue) => setInputValue(newInputValue)}
              onChange={(_, newValue) => {
                setIsEdit(true);
                const newValues = [...new Set(newValue as string[])];
                field.onChange(newValues);
              }}
              renderValue={(value, getTagProps) => {
                return value.map((option, index: number) => {
                  return (
                    <Chip
                      variant='outlined'
                      size='small'
                      label={<Box sx={{ fontSize: '12px' }}>{option}</Box>}
                      {...getTagProps({ index })}
                      key={index}
                    />
                  );
                });
              }}
              renderInput={params => (
                <TextField
                  {...params}
                  size='small'
                  placeholder='选择或输入评价，可多选，回车确认'
                  variant='outlined'
                />
              )}
            />
          )}
        />
      </FormItem>
      <FormItem label='评价开关'>
        <Controller
          control={control}
          name='is_enabled'
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
        />{' '}
      </FormItem>
      <FormItem label='免责声明' permission={PROFESSION_VERSION_PERMISSION}>
        <Controller
          control={control}
          name='disclaimer'
          render={({ field }) => (
            <TextField
              {...field}
              fullWidth
              value={field.value || ''}
              placeholder='请输入免责声明'
              onChange={e => {
                setIsEdit(true);
                field.onChange(e.target.value);
              }}
            ></TextField>
          )}
        />
      </FormItem>
    </SettingCardItem>
  );
};

const DocumentContribution = ({
  data,
  refresh,
}: {
  data: DomainAppDetailResp;
  refresh: () => void;
}) => {
  const [isEdit, setIsEdit] = useState(false);
  const { kb_id } = useAppSelector(state => state.config);
  const { control, handleSubmit, setValue } = useForm({
    defaultValues: {
      is_enable: false,
    },
  });

  const onSubmit = handleSubmit(formData => {
    putApiV1App(
      { id: data.id! },
      {
        kb_id,
        settings: {
          ...data.settings,
          contribute_settings: {
            is_enable: formData.is_enable,
          },
        },
      },
    ).then(() => {
      message.success('保存成功');
      setIsEdit(false);
      refresh();
    });
  });

  useEffect(() => {
    setValue(
      'is_enable',
      // @ts-expect-error 忽略类型错误
      data?.settings?.contribute_settings?.is_enable,
    );
  }, [data]);

  return (
    <SettingCardItem title='文档贡献' isEdit={isEdit} onSubmit={onSubmit}>
      <FormItem label='文档贡献' permission={PROFESSION_VERSION_PERMISSION}>
        <Controller
          control={control}
          name='is_enable'
          render={({ field }) => (
            <RadioGroup
              row
              {...field}
              value={field.value}
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
    </SettingCardItem>
  );
};

const CardFeedback = ({ kb }: CardCommentProps) => {
  const [info, setInfo] = useState<DomainAppDetailResp | null>(null);

  const getInfo = async () => {
    const res = await getApiV1AppDetail({ kb_id: kb.id!, type: '1' });
    setInfo(res);
  };

  useEffect(() => {
    getInfo();
  }, [kb]);

  if (!info) return <></>;

  return (
    <Box
      sx={{
        width: 1000,
        margin: 'auto',
        pb: 4,
      }}
    >
      <AIQuestion data={info} refresh={getInfo} />
      <DocumentComments data={info} refresh={getInfo} />
      <DocumentContribution data={info} refresh={getInfo} />
    </Box>
  );
};

export default CardFeedback;
