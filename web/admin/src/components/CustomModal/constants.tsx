import { lazy } from 'react';
import {
  IconMulushu,
  IconJichuwendang,
  IconJianyiwendang,
  IconChangjianwenti,
  IconLunbotu,
  IconDanwenzi,
  IconShuzikapian,
  IconKehuanli,
  IconTexing,
  IconZuotuyouzi,
  IconYoutuzuozi,
  IconKehupingjia,
  IconJiugongge,
  IconLianjiezu1,
  IconWenjianjia1,
} from '@panda-wiki/icons';
import {
  DomainRecommendNodeListResp,
  GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp,
} from '@/request/types';

export const DEFAULT_DATA = {
  text: {
    title: '标题',
  },
  metrics: {
    title: '指标卡片',
    list: [] as {
      name: string;
      number: string;
      id: string;
    }[],
  },
  case: {
    title: '案例卡片',
    list: [] as {
      id: string;
      name: string;
      link: string;
    }[],
  },
  feature: {
    title: '产品特性',
    list: [] as {
      id: string;
      name: string;
      desc: string;
    }[],
  },
  img_text: {
    title: '图文卡片',
    item: {
      url: '',
      name: '',
      desc: '',
    },
  },
  text_img: {
    title: '图文卡片',
    item: {
      url: '',
      name: '',
      desc: '',
    },
  },
  comment: {
    title: '点评卡片',
    list: [] as {
      id: string;
      user_name: string;
      avatar: string;
      profession: string;
      comment: string;
    }[],
  },
  block_grid: {
    title: '区块网格',
    list: [] as {
      id: string;
      url: string;
      name: string;
    }[],
  },
  banner: {
    title: '',
    subtitle: '',
    placeholder: '',
    bg_url: '',
    hot_search: [] as string[],
    btns: [] as {
      id: string;
      text: string;
      type: 'contained' | 'outlined' | 'text';
      href: string;
    }[],
  },
  basic_doc: {
    title: '文档摘要卡片',
    nodes: [] as DomainRecommendNodeListResp[],
  },
  dir_doc: {
    title: '文件夹卡片',
    nodes: [] as DomainRecommendNodeListResp[],
  },
  nav_doc: {
    title: '目录卡片',
    nodes: [] as (GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp & {
      id: string;
    })[],
  },
  simple_doc: {
    title: '简易文档卡片',
    nodes: [] as DomainRecommendNodeListResp[],
  },
  carousel: {
    title: '轮播图',
    list: [] as {
      id: string;
      title: string;
      url: string;
      desc: string;
    }[],
  },
  faq: {
    title: '链接组',
    list: [] as {
      id: string;
      question: string;
      link: string;
    }[],
  },
  question: {
    title: '常见问题',
    list: [] as {
      id: string;
      question: string;
    }[],
  },
};

export const COMPONENTS_MAP = {
  header: {
    name: 'header',
    title: '顶部导航',
    component: lazy(() => import('@panda-wiki/ui/welcomeHeader')),
    config: lazy(() => import('./components/config/HeaderConfig')),
    fixed: true,
    disabled: false,
    hidden: false,
  },
  banner: {
    name: 'banner',
    title: '欢迎组件',
    component: lazy(() => import('@panda-wiki/ui/banner')),
    config: lazy(() => import('./components/config/BannerConfig')),
    fixed: true,
    disabled: false,
    hidden: false,
  },
  basic_doc: {
    name: 'basic_doc',
    title: '文档摘要卡片',
    icon: IconJichuwendang,
    component: lazy(() => import('@panda-wiki/ui/basicDoc')),
    config: lazy(() => import('./components/config/BasicDocConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  dir_doc: {
    name: 'dir_doc',
    title: '文件夹卡片',
    icon: IconWenjianjia1,
    component: lazy(() => import('@panda-wiki/ui/dirDoc')),
    config: lazy(() => import('./components/config/DirDocConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  nav_doc: {
    name: 'nav_doc',
    title: '目录卡片',
    icon: IconMulushu,
    component: lazy(() => import('@panda-wiki/ui/navDoc')),
    config: lazy(() => import('./components/config/NavDocConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  simple_doc: {
    name: 'simple_doc',
    title: '简易文档卡片',
    icon: IconJianyiwendang,
    component: lazy(() => import('@panda-wiki/ui/simpleDoc')),
    config: lazy(() => import('./components/config/SimpleDocConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  carousel: {
    name: 'carousel',
    title: '轮播图',
    icon: IconLunbotu,
    component: lazy(() => import('@panda-wiki/ui/carousel')),
    config: lazy(() => import('./components/config/CarouselConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  faq: {
    name: 'faq',
    title: '链接组',
    icon: IconLianjiezu1,
    component: lazy(() => import('@panda-wiki/ui/faq')),
    config: lazy(() => import('./components/config/FaqConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  footer: {
    name: 'footer',
    title: '底部导航',
    component: lazy(() => import('@panda-wiki/ui/welcomeFooter')),
    config: lazy(() => import('./components/config/FooterConfig')),
    fixed: true,
    disabled: false,
    hidden: false,
  },
  text: {
    name: 'text',
    title: '标题',
    icon: IconDanwenzi,
    component: lazy(() => import('@panda-wiki/ui/text')),
    config: lazy(() => import('./components/config/TextConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  case: {
    name: 'case',
    title: '案例卡片',
    icon: IconKehuanli,
    component: lazy(() => import('@panda-wiki/ui/case')),
    config: lazy(() => import('./components/config/CaseConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  metrics: {
    name: 'metrics',
    title: '指标卡片',
    icon: IconShuzikapian,
    component: lazy(() => import('@panda-wiki/ui/metrics')),
    config: lazy(() => import('./components/config/MetricsConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  feature: {
    name: 'feature',
    title: '产品特性',
    icon: IconTexing,
    component: lazy(() => import('@panda-wiki/ui/feature')),
    config: lazy(() => import('./components/config/FeatureConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  img_text: {
    name: 'img_text',
    title: '左图右字',
    icon: IconZuotuyouzi,
    component: lazy(() => import('@panda-wiki/ui/imgText')),
    config: lazy(() => import('./components/config/ImgTextConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  text_img: {
    name: 'text_img',
    title: '右图左字',
    icon: IconYoutuzuozi,
    component: lazy(() => import('@panda-wiki/ui/imgText')),
    config: lazy(() => import('./components/config/ImgTextConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  comment: {
    name: 'comment',
    title: '评论卡片',
    icon: IconKehupingjia,
    component: lazy(() => import('@panda-wiki/ui/comment')),
    config: lazy(() => import('./components/config/CommentConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  block_grid: {
    name: 'block_grid',
    title: '区块网格',
    icon: IconJiugongge,
    component: lazy(() => import('@panda-wiki/ui/blockGrid')),
    config: lazy(() => import('./components/config/BlockGridConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
  question: {
    name: 'question',
    title: '常见问题',
    icon: IconChangjianwenti,
    component: lazy(() => import('@panda-wiki/ui/question')),
    config: lazy(() => import('./components/config/QuestionConfig')),
    fixed: false,
    disabled: false,
    hidden: false,
  },
};

export const TYPE_TO_CONFIG_LABEL = {
  banner: 'banner_config',
  basic_doc: 'basic_doc_config',
  dir_doc: 'dir_doc_config',
  nav_doc: 'nav_doc_config',
  simple_doc: 'simple_doc_config',
  carousel: 'carousel_config',
  faq: 'faq_config',
  text: 'text_config',
  case: 'case_config',
  metrics: 'metrics_config',
  feature: 'feature_config',
  text_img: 'text_img_config',
  img_text: 'img_text_config',
  comment: 'comment_config',
  block_grid: 'block_grid_config',
  question: 'question_config',
} as const;
