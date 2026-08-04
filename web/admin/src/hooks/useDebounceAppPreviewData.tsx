import { useAppDispatch } from '@/store';
import { setAppPreviewData } from '@/store/slices/config';
import { debounce } from 'lodash-es';
import { useEffect, useMemo } from 'react';

const useDebounceAppPreviewData = () => {
  const dispatch = useAppDispatch();

  const debouncedDispatch = useMemo(
    () =>
      debounce((data: any) => {
        dispatch(setAppPreviewData(data));
      }, 500),
    [dispatch],
  );

  useEffect(() => {
    return () => {
      debouncedDispatch.cancel();
    };
  }, [debouncedDispatch]);

  return debouncedDispatch;
};

export default useDebounceAppPreviewData;
