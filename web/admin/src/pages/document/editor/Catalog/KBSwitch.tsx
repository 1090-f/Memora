import { useAppSelector } from '@/store';
import { Ellipsis } from '@ctzhian/ui';
import { Stack } from '@mui/material';
import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { IconZuzhi } from '@panda-wiki/icons';

const KBSwitch = () => {
  // const dispatch = useAppDispatch();
  const navigate = useNavigate();

  const { kbList, kb_id } = useAppSelector(state => state.config);

  const currentKb = useMemo(() => {
    return kbList?.find(item => item.id === kb_id);
  }, [kbList, kb_id]);

  // const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);

  // const handlePopoverOpen = (event: React.MouseEvent<HTMLElement>) => {
  //   setAnchorEl(event.currentTarget);
  // };

  // const handlePopoverClose = () => {
  //   setAnchorEl(null);
  // };

  // const open = Boolean(anchorEl);

  return (
    <Stack direction='row' alignItems='center' gap={1} sx={{ flex: 1 }}>
      <Stack
        aria-describedby={'editor-kb-switch'}
        alignItems='center'
        justifyContent={'center'}
        sx={{
          cursor: 'pointer',
          flexShrink: 0,
          bgcolor: 'background.paper',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '10px',
          width: 36,
          height: 36,
        }}
        onClick={() => {
          navigate('/');
        }}
      >
        <IconZuzhi type='icon-zuzhi' sx={{ color: 'text.primary' }} />
      </Stack>
      <Ellipsis sx={{ flex: 1, width: 0, overflow: 'hidden' }}>
        {currentKb?.name}
      </Ellipsis>
      {/* <Popover
        id='editor-kb-switch'
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
        onClose={handlePopoverClose}
        disableAutoFocus
      >
        <Stack sx={{ width: 200 }}>
          <Box sx={{ px: 2, pt: 1.5, pb: 1, fontWeight: 'bold', fontSize: 12 }}>
            全部知识库
          </Box>
          <Divider />
          <Stack sx={{ p: 0.5 }}>
            {kbList.map(item => (
              <MenuItem
                key={item.id}
                selected={item.id === kb_id}
                onClick={() => {
                  dispatch(setKbId(item.id));
                  handlePopoverClose();
                  navigate(`/doc/editor/space?id=${item.id}`);
                }}
              >
                {item.name}
              </MenuItem>
            ))}
          </Stack>
        </Stack>
      </Popover> */}
    </Stack>
  );
};

export default KBSwitch;
