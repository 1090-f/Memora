import {
  FeatureStatus,
  VersionInfoMap,
  VersionInfo,
  getFeatureValue,
} from '@/constant/version';
import { ConstsLicenseEdition } from '@/request/types';
import { useAppSelector } from '@/store';

export const useFeatureValue = <K extends keyof VersionInfo['features']>(
  key: K,
): VersionInfo['features'][K] => {
  const { license } = useAppSelector(state => state.config);
  return getFeatureValue(license.edition!, key);
};

export const useFeatureValueSupported = (
  key: keyof VersionInfo['features'],
) => {
  const { license } = useAppSelector(state => state.config);
  return (
    getFeatureValue(license.edition!, key) === FeatureStatus.SUPPORTED ||
    getFeatureValue(license.edition!, key) === FeatureStatus.ADVANCED
  );
};

export const useVersionInfo = () => {
  const { license } = useAppSelector(state => state.config);
  return (
    VersionInfoMap[
      license.edition ?? ConstsLicenseEdition.LicenseEditionFree
    ] || VersionInfoMap[ConstsLicenseEdition.LicenseEditionFree]
  );
};
