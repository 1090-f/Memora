import {
  AnydocDingtalkSetting,
  AnydocFeishuSetting,
  ConstsCrawlerSource,
} from '@/request';
import { useAppSelector } from '@/store';
import { Modal } from '@ctzhian/ui';
import { useCallback, useMemo, useState } from 'react';
import { TYPE_CONFIG, UPLOAD_FILE_TYPES } from './constants';
import FileParse from './FileParse';
import FormSubmit from './FormSubmit';
import { useGlobalQueue } from './hooks/useGlobalQueue';
import ListRender from './ListRender';

interface AddDocByTypeProps {
  open: boolean;
  refresh?: () => void;
  onCancel: () => void;
  parentId: string | null;
  type: ConstsCrawlerSource;
}

export interface ListDataItem {
  uuid: string;
  task_id?: string;
  space_id?: string;
  parent_id?: string;
  platform_id?: string;
  id?: string;

  title?: string;
  summary?: string;
  file_type?: string;
  file?: boolean;
  fileData?: File;
  progress?: number;

  open?: boolean;
  folderReq?: boolean;

  feishu_setting?: AnydocFeishuSetting;
  dingtalk_setting?: AnydocDingtalkSetting;

  status:
    | 'common'
    | 'upload-error'
    | 'parsing'
    | 'parsed'
    | 'parse-error'
    | 'importing'
    | 'imported'
    | 'import-error';
}

const AddDocByType = ({
  type,
  open,
  refresh,
  onCancel,
  parentId = null,
}: AddDocByTypeProps) => {
  const { kb_id } = useAppSelector(state => state.config);
  const [data, setData] = useState<ListDataItem[]>([]);
  const [checked, setChecked] = useState<string[]>([]);
  const [formSubmitLoading, setFormSubmitLoading] = useState(false);

  const queue = useGlobalQueue(5);

  const isUploadFileType = useMemo(() => {
    return UPLOAD_FILE_TYPES.includes(type);
  }, [type]);

  const isSupportSelect = useMemo(() => {
    return [
      ConstsCrawlerSource.CrawlerSourceRSS,
      ConstsCrawlerSource.CrawlerSourceSitemap,
      ConstsCrawlerSource.CrawlerSourceDingtalk,
      ConstsCrawlerSource.CrawlerSourceFeishu,
    ].includes(type);
  }, [type]);

  const handleCancel = useCallback(() => {
    onCancel();
    if (data.some(item => item.status === 'imported')) {
      refresh?.();
    }
    setData([]);
    setChecked([]);
  }, [onCancel, refresh, data]);

  return (
    <Modal
      open={open}
      width={900}
      disableEscapeKeyDown
      onCancel={handleCancel}
      title={TYPE_CONFIG[type].label}
      footer={null}
    >
      {data.length > 0 ? (
        <>
          <ListRender
            data={data}
            setData={setData}
            isSupportSelect={isSupportSelect}
            checked={checked}
            parent_id={parentId}
            setChecked={setChecked}
            loading={formSubmitLoading}
            type={type}
            queue={queue}
          />
        </>
      ) : isUploadFileType ? (
        <>
          <FileParse type={type} parent_id={parentId} setData={setData} />
        </>
      ) : (
        <>
          <FormSubmit
            type={type}
            kb_id={kb_id}
            parent_id={parentId}
            setData={setData}
            loading={formSubmitLoading}
            setLoading={setFormSubmitLoading}
            queue={queue}
          />
        </>
      )}
    </Modal>
  );
};

export default AddDocByType;
