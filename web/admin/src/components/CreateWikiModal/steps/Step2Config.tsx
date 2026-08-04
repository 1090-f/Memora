import React, { useState, useImperativeHandle, Ref, useEffect } from 'react';
import {
  Checkbox,
  FormControlLabel,
  TextField,
  Typography,
  Stack,
  FormControl,
  FormHelperText,
} from '@mui/material';
import {
  getApiV1KnowledgeBaseList,
  getApiV1KnowledgeBaseDetail,
  postApiV1KnowledgeBase,
} from '@/request/KnowledgeBase';
import { DomainCreateKnowledgeBaseReq } from '@/request/types';
import { setKbId, setKbList, setKbDetail } from '@/store/slices/config';
import { SettingCardItem, FormItem } from '@/pages/setting/component/Common';
import { Controller, useForm } from 'react-hook-form';
import FileText from '@/components/UploadFile/FileText';
import { message } from '@ctzhian/ui';
import { useAppDispatch } from '@/store';

const VALIDATION_RULES = {
  name: {
    required: {
      value: true,
      message: 'Wiki 站名称不能为空',
    },
  },
  port: {
    required: {
      value: true,
      message: '端口不能为空',
    },
    min: {
      value: 1,
      message: '端口号不能小于1',
    },
    max: {
      value: 65535,
      message: '端口号不能大于65535',
    },
  },
  domain: {
    pattern: {
      value:
        /^(localhost|((([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)\.)+[a-zA-Z]{2,})|(\d{1,3}(?:\.\d{1,3}){3})|(\[[0-9a-fA-F:]+\]))$/,
      message: '请输入有效的域名、IP 或 localhost',
    },
  },
  http: {
    validate: (
      value: boolean,
      formValues: { http: boolean; https: boolean },
    ) => {
      if (!value && !formValues.https) {
        return 'HTTP 端口和 HTTPS 端口必须有一个启用';
      }
      return true;
    },
  },
  https: {
    validate: (
      value: boolean,
      formValues: { http: boolean; https: boolean },
    ) => {
      if (!value && !formValues.http) {
        return 'HTTP 端口和 HTTPS 端口必须有一个启用';
      }
      return true;
    },
  },
};

interface Step2ConfigProps {
  ref: Ref<{ onSubmit: () => Promise<unknown> }>;
}

const Step2Config: React.FC<Step2ConfigProps> = ({ ref }) => {
  const {
    control,
    formState: { errors },
    trigger,
    watch,
    reset,
    getValues,
  } = useForm({
    defaultValues: {
      name: '',
      domain: window.location.hostname,
      port: 80,
      ssl_port: 443,
      httpsCert: '',
      httpsKey: '',
      http: true,
      https: false,
    },
  });

  const { http, https } = watch();

  useEffect(() => {
    return () => {
      reset();
    };
  }, []);

  const dispatch = useAppDispatch();

  const getKb = (id?: string) => {
    const kb_id = id || localStorage.getItem('kb_id') || '';
    return Promise.all([
      getApiV1KnowledgeBaseList().then(res => {
        dispatch(setKbList(res));
        if (res.find(item => item.id === kb_id)) {
          dispatch(setKbId(kb_id));
        } else {
          dispatch(setKbId(res[0]?.id || ''));
        }
      }),
      getApiV1KnowledgeBaseDetail({ id: kb_id }).then(res => {
        dispatch(setKbDetail(res));
      }),
    ]);
  };

  const onSubmit = async () => {
    const isRHFValid = await trigger();
    if (!isRHFValid) {
      return Promise.reject();
    } else {
      const value = getValues();
      if (!value.http && !value.https) {
        message.error('HTTP 和 HTTPS 至少需要启用一种服务');
        return Promise.reject(new Error('HTTP 和 HTTPS 至少需要启用一种服务'));
      }
      const formData: DomainCreateKnowledgeBaseReq = { name: value.name };
      if (value.domain) formData.hosts = [value.domain];
      if (value.http) formData.ports = [+value.port];
      if (value.https) {
        formData.ssl_ports = [+value.ssl_port];
        if (value.httpsCert) formData.public_key = value.httpsCert;
        if (value.httpsKey) formData.private_key = value.httpsKey;
      }

      return (
        postApiV1KnowledgeBase(formData)
          // @ts-expect-error 类型错误
          .then(({ id }) => {
            return getKb(id).then(() => {
              // message.success('创建成功');
            });
          })
      );
    }
  };

  useImperativeHandle(ref, () => ({
    onSubmit,
  }));

  return (
    <>
      <SettingCardItem title='WIKI 站'>
        {/* Knowledge Base Name Section */}
        <FormItem
          label='名称'
          required
          labelWidth={100}
          sx={{ alignItems: 'flex-start' }}
          labelSx={{ height: 52 }}
        >
          <Controller
            control={control}
            name='name'
            rules={VALIDATION_RULES.name}
            render={({ field }) => (
              <TextField
                {...field}
                autoFocus
                placeholder='请输入'
                fullWidth
                error={!!errors.name}
                helperText={errors.name?.message}
              />
            )}
          />
        </FormItem>
      </SettingCardItem>
      <SettingCardItem title='服务监听方式'>
        <FormItem
          label='域名或 IP'
          labelWidth={100}
          sx={{ alignItems: 'flex-start' }}
          labelSx={{ height: 52 }}
        >
          <Controller
            control={control}
            name='domain'
            rules={VALIDATION_RULES.domain}
            render={({ field }) => (
              <TextField
                {...field}
                fullWidth
                placeholder='请输入'
                error={!!errors.domain}
                helperText={errors.domain?.message}
              />
            )}
          />
        </FormItem>
        <FormItem
          label='HTTP 端口'
          labelWidth={100}
          sx={{ alignItems: 'flex-start' }}
          labelSx={{ height: 52 }}
        >
          <Stack direction='row' gap={2} sx={{ flex: 1 }}>
            <FormControl error={!!errors.http}>
              <Controller
                control={control}
                name='http'
                // rules={VALIDATION_RULES.http}
                render={({ field }) => (
                  <FormControlLabel
                    sx={{ mr: 0, height: 52 }}
                    control={
                      <Checkbox
                        checked={field.value}
                        onChange={e => field.onChange(e.target.checked)}
                        sx={{ padding: '4px' }}
                      />
                    }
                    label={
                      <Typography sx={{ fontSize: '14px', minWidth: '30px' }}>
                        启用
                      </Typography>
                    }
                  />
                )}
              />
              {/* {errors.http && (
                <FormHelperText>{errors.http.message}</FormHelperText>
              )} */}
            </FormControl>

            <Controller
              control={control}
              name='port'
              rules={VALIDATION_RULES.port}
              render={({ field }) => (
                <TextField
                  {...field}
                  placeholder='HTTP 端口'
                  disabled={!http}
                  fullWidth
                  error={!!errors.port}
                  helperText={errors.port?.message}
                />
              )}
            />
          </Stack>
        </FormItem>
        <FormItem
          label='HTTPS 端口'
          labelWidth={100}
          sx={{ alignItems: 'flex-start' }}
          labelSx={{ height: 52 }}
        >
          <Stack direction='row' gap={2} sx={{ flex: 1 }}>
            <FormControl error={!!errors.https}>
              <Controller
                control={control}
                name='https'
                // rules={VALIDATION_RULES.https}
                render={({ field }) => (
                  <FormControlLabel
                    sx={{ mr: 0, height: 52 }}
                    control={
                      <Checkbox
                        checked={field.value}
                        onChange={e => field.onChange(e.target.checked)}
                        sx={{ padding: '4px' }}
                      />
                    }
                    label={
                      <Typography sx={{ fontSize: '14px', minWidth: '30px' }}>
                        启用
                      </Typography>
                    }
                  />
                )}
              />
              {/* {errors.https && (
                <FormHelperText>{errors.https.message}</FormHelperText>
              )} */}
            </FormControl>

            <Controller
              control={control}
              name='ssl_port'
              rules={VALIDATION_RULES.port}
              render={({ field }) => (
                <TextField
                  {...field}
                  placeholder='HTTPS 端口'
                  disabled={!https}
                  sx={{ width: 137 }}
                  error={!!errors.ssl_port}
                  helperText={errors.ssl_port?.message}
                />
              )}
            />

            <FormControl error={!!errors.httpsCert} sx={{ width: 137 }}>
              <Controller
                control={control}
                name='httpsCert'
                rules={{ required: https ? '请上传' : false }}
                render={({ field }) => (
                  <FileText
                    {...field}
                    sx={{ width: 137 }}
                    textSx={{ fontSize: 14 }}
                    tip={'证书文件'}
                    disabled={!https}
                  />
                )}
              />
              {errors.httpsCert && (
                <FormHelperText>{errors.httpsCert.message}</FormHelperText>
              )}
            </FormControl>
            <FormControl error={!!errors.httpsKey} sx={{ width: 137 }}>
              <Controller
                control={control}
                name='httpsKey'
                rules={{ required: https ? '请上传' : false }}
                render={({ field }) => (
                  <FileText
                    {...field}
                    sx={{ width: 137 }}
                    textSx={{ fontSize: 14 }}
                    tip={'私钥文件'}
                    disabled={!https}
                  />
                )}
              />
              {errors.httpsKey && (
                <FormHelperText>{errors.httpsKey.message}</FormHelperText>
              )}
            </FormControl>
          </Stack>
        </FormItem>
      </SettingCardItem>
    </>
  );
};

export default Step2Config;
