import { updateKnowledgeBase } from '@/api';
import { DomainKnowledgeBaseDetail } from '@/request/types';
import { SettingCardItem, FormItem } from './Common';

import {
  Box,
  FormControl,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  TextField,
} from '@mui/material';
import { message } from '@ctzhian/ui';
import { useEffect, useState } from 'react';

const CardProxy = ({
  kb,
  refresh,
}: {
  kb: DomainKnowledgeBaseDetail;
  refresh: () => void;
}) => {
  const [isEdit, setIsEdit] = useState(false);
  const [hasProxy, setHasProxy] = useState(
    !!kb.access_settings?.trusted_proxies?.length,
  );
  const [proxyIPs, setProxyIPs] = useState<string[]>(
    kb.access_settings?.trusted_proxies || [],
  );

  const handleSave = () => {
    try {
      updateKnowledgeBase({
        id: kb.id,
        access_settings: {
          ...kb.access_settings,
          trusted_proxies: hasProxy
            ? proxyIPs.filter(ip => ip.trim() !== '')
            : null,
        },
      }).then(() => {
        message.success('保存成功');
        setIsEdit(false);
        refresh();
      });
    } catch (e) {
      message.error('保存失败');
    }
  };

  useEffect(() => {
    setHasProxy(!!kb.access_settings?.trusted_proxies?.length);
    setProxyIPs(kb.access_settings?.trusted_proxies || []);
  }, [kb]);

  return (
    <SettingCardItem
      title='前置反向代理'
      more={
        <Box
          sx={{
            flexGrow: 1,
            fontSize: 12,
            color: 'text.tertiary',
            ml: 1,
            fontWeight: 'normal',
          }}
        >
          用于修正源 IP 获取错误的问题
        </Box>
      }
      isEdit={isEdit}
      onSubmit={handleSave}
    >
      <FormItem label='前置反向代理'>
        <FormControl>
          <RadioGroup
            value={hasProxy}
            onChange={e => {
              setHasProxy(e.target.value === 'true');
              if (proxyIPs.length === 0) {
                setProxyIPs(['0.0.0.0/0']);
              }
              setIsEdit(true);
            }}
          >
            <Stack direction={'row'}>
              <FormControlLabel
                value={false}
                control={<Radio size='small' />}
                label='无前置反向代理'
              />
              <FormControlLabel
                value={true}
                control={<Radio size='small' />}
                label='有前置反向代理'
              />
            </Stack>
          </RadioGroup>
        </FormControl>
      </FormItem>

      {hasProxy && (
        <FormItem label='可信代理列表' sx={{ alignItems: 'flex-start' }}>
          <TextField
            fullWidth
            label='可信代理 IP 或 CIDR（换行分隔）'
            multiline
            minRows={2}
            value={proxyIPs.join('\n')}
            helperText='支持填写多个 IP 或 CIDR，每行一个'
            onChange={e => {
              const lines = e.target.value.split(/\r?\n/).map(s => s.trim());
              setProxyIPs(lines);
              setIsEdit(true);
            }}
          />
        </FormItem>
      )}
    </SettingCardItem>
  );
};

export default CardProxy;
