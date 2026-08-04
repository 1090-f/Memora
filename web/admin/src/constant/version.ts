import { ConstsLicenseEdition } from '@/request/types';

import freeVersion from '@/assets/images/free-version.png';
import proVersion from '@/assets/images/pro-version.png';
import businessVersion from '@/assets/images/business-version.png';
import enterpriseVersion from '@/assets/images/enterprise-version.png';

export const PROFESSION_VERSION_PERMISSION = [
  ConstsLicenseEdition.LicenseEditionProfession,
  ConstsLicenseEdition.LicenseEditionBusiness,
  ConstsLicenseEdition.LicenseEditionEnterprise,
];

export const BUSINESS_VERSION_PERMISSION = [
  ConstsLicenseEdition.LicenseEditionBusiness,
  ConstsLicenseEdition.LicenseEditionEnterprise,
];

export const ENTERPRISE_VERSION_PERMISSION = [
  ConstsLicenseEdition.LicenseEditionEnterprise,
];

export const VersionInfoMap = {
  [ConstsLicenseEdition.LicenseEditionFree]: {
    permission: ConstsLicenseEdition.LicenseEditionFree,
    label: '开源版',
    image: freeVersion,
    bgColor: '#8E9DAC',
    nextVersion: ConstsLicenseEdition.LicenseEditionProfession,
  },
  [ConstsLicenseEdition.LicenseEditionProfession]: {
    permission: ConstsLicenseEdition.LicenseEditionProfession,
    label: '专业版',
    image: proVersion,
    bgColor: '#0933BA',
    nextVersion: ConstsLicenseEdition.LicenseEditionBusiness,
  },
  [ConstsLicenseEdition.LicenseEditionBusiness]: {
    permission: ConstsLicenseEdition.LicenseEditionBusiness,
    label: '商业版',
    image: businessVersion,
    bgColor: '#382A79',
    nextVersion: ConstsLicenseEdition.LicenseEditionEnterprise,
  },
  [ConstsLicenseEdition.LicenseEditionEnterprise]: {
    permission: ConstsLicenseEdition.LicenseEditionEnterprise,
    label: '企业版',
    image: enterpriseVersion,
    bgColor: '#21222D',
    nextVersion: undefined,
  },
};

/**
 * 功能支持状态
 */
export enum FeatureStatus {
  /** 不支持 */
  NOT_SUPPORTED = 'not_supported',
  /** 支持 */
  SUPPORTED = 'supported',
  /** 基础配置 */
  BASIC = 'basic',
  /** 高级配置 */
  ADVANCED = 'advanced',
}

/**
 * 版本信息配置
 */
export interface VersionInfo {
  /** 版本名称 */
  label: string;
  /** 功能特性 */
  features: {
    /** Wiki 站点数量 */
    wikiCount: number;
    /** 每个 Wiki 的文档数量 */
    docCountPerWiki: number;
    /** 管理员数量 */
    adminCount: number;
    /** 管理员分权控制 */
    adminPermissionControl: FeatureStatus;
    /** SEO 配置 */
    seoConfig: FeatureStatus;
    /** 多语言支持 */
    multiLanguage: FeatureStatus;
    /** 自定义版权信息 */
    customCopyright: FeatureStatus;
    /** 访问流量分析 */
    trafficAnalysis: FeatureStatus;
    /** 自定义 AI 提示词 */
    customAIPrompt: FeatureStatus;
    /** SSO 登录 */
    ssoLogin: number;
    /** 访客权限控制 */
    visitorPermissionControl: FeatureStatus;
    /** 页面水印 */
    pageWatermark: FeatureStatus;
    /** 内容不可复制 */
    contentNoCopy: FeatureStatus;
    /** 敏感内容过滤 */
    sensitiveContentFilter: FeatureStatus;
    /** 网页挂件机器人 */
    webWidgetRobot: FeatureStatus;
    /** 飞书问答机器人 */
    feishuQARobot: FeatureStatus;
    /** 钉钉问答机器人 */
    dingtalkQARobot: FeatureStatus;
    /** 企业微信问答机器人 */
    wecomQARobot: FeatureStatus;
    /** 企业微信客服机器人 */
    wecomServiceRobot: FeatureStatus;
    /** Discord 问答机器人 */
    discordQARobot: FeatureStatus;
    /** 文档历史版本管理 */
    docVersionHistory: FeatureStatus;
    /** API 调用 */
    apiCall: FeatureStatus;
    /** 项目源码 */
    sourceCode: FeatureStatus;
  };
}

/**
 * 版本信息映射
 */
export const VERSION_INFO: Record<ConstsLicenseEdition, VersionInfo> = {
  [ConstsLicenseEdition.LicenseEditionFree]: {
    label: '开源版',
    features: {
      wikiCount: 1,
      docCountPerWiki: 300,
      adminCount: 1,
      adminPermissionControl: FeatureStatus.NOT_SUPPORTED,
      seoConfig: FeatureStatus.BASIC,
      multiLanguage: FeatureStatus.NOT_SUPPORTED,
      customCopyright: FeatureStatus.NOT_SUPPORTED,
      trafficAnalysis: FeatureStatus.BASIC,
      customAIPrompt: FeatureStatus.NOT_SUPPORTED,
      ssoLogin: 0,
      visitorPermissionControl: FeatureStatus.NOT_SUPPORTED,
      pageWatermark: FeatureStatus.NOT_SUPPORTED,
      contentNoCopy: FeatureStatus.NOT_SUPPORTED,
      sensitiveContentFilter: FeatureStatus.NOT_SUPPORTED,
      webWidgetRobot: FeatureStatus.BASIC,
      feishuQARobot: FeatureStatus.BASIC,
      dingtalkQARobot: FeatureStatus.BASIC,
      wecomQARobot: FeatureStatus.BASIC,
      wecomServiceRobot: FeatureStatus.BASIC,
      discordQARobot: FeatureStatus.BASIC,
      docVersionHistory: FeatureStatus.NOT_SUPPORTED,
      apiCall: FeatureStatus.NOT_SUPPORTED,
      sourceCode: FeatureStatus.SUPPORTED,
    },
  },
  [ConstsLicenseEdition.LicenseEditionProfession]: {
    label: '专业版',
    features: {
      wikiCount: 10,
      docCountPerWiki: 10000,
      adminCount: 20,
      adminPermissionControl: FeatureStatus.SUPPORTED,
      seoConfig: FeatureStatus.ADVANCED,
      multiLanguage: FeatureStatus.SUPPORTED,
      customCopyright: FeatureStatus.SUPPORTED,
      trafficAnalysis: FeatureStatus.ADVANCED,
      customAIPrompt: FeatureStatus.SUPPORTED,
      ssoLogin: 0,
      visitorPermissionControl: FeatureStatus.NOT_SUPPORTED,
      pageWatermark: FeatureStatus.NOT_SUPPORTED,
      contentNoCopy: FeatureStatus.NOT_SUPPORTED,
      sensitiveContentFilter: FeatureStatus.NOT_SUPPORTED,
      webWidgetRobot: FeatureStatus.ADVANCED,
      feishuQARobot: FeatureStatus.ADVANCED,
      dingtalkQARobot: FeatureStatus.ADVANCED,
      wecomQARobot: FeatureStatus.ADVANCED,
      wecomServiceRobot: FeatureStatus.ADVANCED,
      discordQARobot: FeatureStatus.ADVANCED,
      docVersionHistory: FeatureStatus.NOT_SUPPORTED,
      apiCall: FeatureStatus.NOT_SUPPORTED,
      sourceCode: FeatureStatus.NOT_SUPPORTED,
    },
  },
  [ConstsLicenseEdition.LicenseEditionBusiness]: {
    label: '商业版',
    features: {
      wikiCount: 20,
      docCountPerWiki: 10000,
      adminCount: 50,
      adminPermissionControl: FeatureStatus.SUPPORTED,
      seoConfig: FeatureStatus.ADVANCED,
      multiLanguage: FeatureStatus.SUPPORTED,
      customCopyright: FeatureStatus.SUPPORTED,
      trafficAnalysis: FeatureStatus.ADVANCED,
      customAIPrompt: FeatureStatus.SUPPORTED,
      ssoLogin: 2000,
      visitorPermissionControl: FeatureStatus.SUPPORTED,
      pageWatermark: FeatureStatus.SUPPORTED,
      contentNoCopy: FeatureStatus.SUPPORTED,
      sensitiveContentFilter: FeatureStatus.SUPPORTED,
      webWidgetRobot: FeatureStatus.ADVANCED,
      feishuQARobot: FeatureStatus.ADVANCED,
      dingtalkQARobot: FeatureStatus.ADVANCED,
      wecomQARobot: FeatureStatus.ADVANCED,
      wecomServiceRobot: FeatureStatus.ADVANCED,
      discordQARobot: FeatureStatus.ADVANCED,
      docVersionHistory: FeatureStatus.SUPPORTED,
      apiCall: FeatureStatus.SUPPORTED,
      sourceCode: FeatureStatus.NOT_SUPPORTED,
    },
  },
  [ConstsLicenseEdition.LicenseEditionEnterprise]: {
    label: '企业版',
    features: {
      wikiCount: Infinity,
      docCountPerWiki: Infinity,
      adminCount: Infinity,
      adminPermissionControl: FeatureStatus.SUPPORTED,
      seoConfig: FeatureStatus.ADVANCED,
      multiLanguage: FeatureStatus.SUPPORTED,
      customCopyright: FeatureStatus.SUPPORTED,
      trafficAnalysis: FeatureStatus.ADVANCED,
      customAIPrompt: FeatureStatus.SUPPORTED,
      ssoLogin: Infinity,
      visitorPermissionControl: FeatureStatus.SUPPORTED,
      pageWatermark: FeatureStatus.SUPPORTED,
      contentNoCopy: FeatureStatus.SUPPORTED,
      sensitiveContentFilter: FeatureStatus.SUPPORTED,
      webWidgetRobot: FeatureStatus.ADVANCED,
      feishuQARobot: FeatureStatus.ADVANCED,
      dingtalkQARobot: FeatureStatus.ADVANCED,
      wecomQARobot: FeatureStatus.ADVANCED,
      wecomServiceRobot: FeatureStatus.ADVANCED,
      discordQARobot: FeatureStatus.ADVANCED,
      docVersionHistory: FeatureStatus.SUPPORTED,
      apiCall: FeatureStatus.SUPPORTED,
      sourceCode: FeatureStatus.SUPPORTED,
    },
  },
};

/**
 * 功能特性标签映射
 */
export const FEATURE_LABELS: Record<string, string> = {
  wikiCount: 'Wiki 站点数量',
  docCountPerWiki: '每个 Wiki 的文档数量',
  adminCount: '管理员数量',
  adminPermissionControl: '管理员分权控制',
  seoConfig: 'SEO 配置',
  multiLanguage: '多语言支持',
  customCopyright: '自定义版权信息',
  trafficAnalysis: '访问流量分析',
  customAIPrompt: '自定义 AI 提示词',
  ssoLogin: 'SSO 登录',
  visitorPermissionControl: '访客权限控制',
  pageWatermark: '页面水印',
  contentNoCopy: '内容不可复制',
  sensitiveContentFilter: '敏感内容过滤',
  webWidgetRobot: '网页挂件机器人',
  feishuQARobot: '飞书问答机器人',
  dingtalkQARobot: '钉钉问答机器人',
  wecomQARobot: '企业微信问答机器人',
  wecomServiceRobot: '企业微信客服机器人',
  discordQARobot: 'Discord 问答机器人',
  docVersionHistory: '文档历史版本管理',
  apiCall: 'API 调用',
  sourceCode: '项目源码',
};

/**
 * 功能状态显示文本映射
 */
export const FEATURE_STATUS_LABELS: Record<FeatureStatus, string> = {
  [FeatureStatus.NOT_SUPPORTED]: '不支持',
  [FeatureStatus.SUPPORTED]: '支持',
  [FeatureStatus.BASIC]: '基础配置',
  [FeatureStatus.ADVANCED]: '高级配置',
};

/**
 * 获取功能特性值
 */
export function getFeatureValue<K extends keyof VersionInfo['features']>(
  edition: ConstsLicenseEdition,
  key: K,
): VersionInfo['features'][K] {
  return (
    VERSION_INFO[edition] ||
    VERSION_INFO[ConstsLicenseEdition.LicenseEditionFree]
  ).features[key];
}
