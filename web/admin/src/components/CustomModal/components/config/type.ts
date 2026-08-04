import { Dispatch, SetStateAction } from 'react';

export interface ConfigProps {
  data?: any | null;
  setIsEdit: Dispatch<SetStateAction<boolean>>;
  isEdit: boolean;
  id: string;
}
