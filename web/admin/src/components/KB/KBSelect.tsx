import { KnowledgeBaseListItem } from '@/api';
import { useURLSearchParams } from '@/hooks';
import { useFeatureValue } from '@/hooks';
import { ConstsUserRole } from '@/request/types';
import { useAppDispatch, useAppSelector } from '@/store';
import { setKbC, setKbId } from '@/store/slices/config';
import { Ellipsis, message } from '@ctzhian/ui';
import {
  IconXiala,
  IconZuzhi,
  IconTianjiawendang,
  IconShanchu,
} from '@panda-wiki/icons';
import {
  Box,
  Button,
  IconButton,
  MenuItem,
  Select,
  Stack,
} from '@mui/material';
import { useState } from 'react';
import { useLocation } from 'react-router-dom';
import KBDelete from './KBDelete';
import KBModify from './KBModify';

const KBSelect = () => {
  const location = useLocation();
  const resetPagination = location.pathname.includes('/conversation');

  const dispatch = useAppDispatch();
  const [_, setSearchParams] = useURLSearchParams();
  const { kb_id, kbList, user } = useAppSelector(state => state.config);

  const [modifyOpen, setModifyOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [opraData, setOpraData] = useState<KnowledgeBaseListItem | null>(null);

  const wikiCount = useFeatureValue('wikiCount');

  return (
    <>
      {(kbList || []).length > 0 && (
        <Select
          value={kb_id}
          size='small'
          sx={{
            maxWidth: 300,
            pr: 2,
            height: 32,
            fontSize: 14,
            fontFamily: 'G',
            transition: 'all 0.3s',
            '.MuiSelect-select': {
              width: 'calc(100% + 48px)',
            },
            '&:hover': {
              transition: 'all 0.3s',
              '.icon-xiala': {
                display: 'block',
              },
            },
            '&.Mui-focused': {
              transition: 'all 0.3s',
              '.icon-xiala': {
                display: 'block',
              },
            },
          }}
          onChange={e => {
            if (e.target.value === kb_id || !e.target.value) return;
            dispatch(setKbId(e.target.value as string));
            if (resetPagination) setSearchParams({ page: '1', pageSize: '20' });
            message.success('切换成功');
          }}
          IconComponent={({ className, ...rest }) => {
            return (
              <IconXiala
                className={className + ' icon-xiala'}
                sx={{
                  position: 'absolute',
                  right: 0,
                  fontSize: 20,
                  flexShrink: 0,
                  mr: 1,
                  transform: className?.includes('MuiSelect-iconOpen')
                    ? 'rotate(-180deg)'
                    : 'none',
                  transition: 'transform 0.3s',
                  cursor: 'pointer',
                  pointerEvents: 'none',
                  display: className?.includes('MuiSelect-iconOpen')
                    ? 'block'
                    : 'none',
                }}
                {...rest}
              />
            );
          }}
          MenuProps={{
            PaperProps: {
              sx: {
                width: 300,
                maxHeight: 292,
              },
            },
            anchorOrigin: {
              vertical: 'bottom',
              horizontal: 'center',
            },
            transformOrigin: {
              vertical: 'top',
              horizontal: 'center',
            },
          }}
        >
          <Button
            size='small'
            sx={{
              height: 40,
              mb: 0.5,
              borderRadius: '5px',
              bgcolor: 'background.paper3',
              '&:hover': {
                bgcolor: 'rgba(50,72,242,0.1)',
              },
            }}
            fullWidth
            disabled={
              (kbList || []).length >= wikiCount ||
              user.role === ConstsUserRole.UserRoleUser
            }
            onClick={event => {
              event.stopPropagation();
              dispatch(setKbC(true));
            }}
          >
            创建新 Wiki 站
          </Button>
          {(kbList || []).map(item => (
            <MenuItem
              key={item.id}
              value={item.id}
              sx={{
                fontFamily: 'G',
                height: '40px',
                '&:hover .hover-del-space-icon': { display: 'inline-flex' },
              }}
            >
              <Stack
                direction={'row'}
                alignItems={'center'}
                gap={1.5}
                sx={{ width: '100%' }}
              >
                <IconZuzhi
                  sx={{ fontSize: 14, color: 'text.secondary', flexShrink: 0 }}
                />
                <Ellipsis>{item.name}</Ellipsis>
                <Box sx={{ width: 10 }}></Box>
                {user.role !== ConstsUserRole.UserRoleUser && (
                  <Stack direction={'row'} alignItems={'center'}>
                    <IconButton
                      size='small'
                      className='hover-del-space-icon'
                      sx={{ display: 'none' }}
                      onClick={event => {
                        event.stopPropagation();
                        setOpraData(item);
                        setModifyOpen(true);
                      }}
                    >
                      <IconTianjiawendang
                        sx={{
                          fontSize: 14,
                          color: 'text.tertiary',
                          flexShrink: 0,
                        }}
                      />
                    </IconButton>
                    <IconButton
                      size='small'
                      className='hover-del-space-icon'
                      sx={{ display: 'none' }}
                      onClick={event => {
                        event.stopPropagation();
                        setOpraData(item);
                        setDeleteOpen(true);
                      }}
                    >
                      <IconShanchu
                        sx={{
                          fontSize: 14,
                          color: 'text.tertiary',
                          flexShrink: 0,
                        }}
                      />
                    </IconButton>
                  </Stack>
                )}
              </Stack>
            </MenuItem>
          ))}
        </Select>
      )}
      <KBDelete
        open={deleteOpen}
        data={opraData}
        onClose={() => setDeleteOpen(false)}
      />
      <KBModify
        open={modifyOpen}
        data={opraData}
        onClose={() => setModifyOpen(false)}
      />
    </>
  );
};

export default KBSelect;
