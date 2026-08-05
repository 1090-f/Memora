import {
  bluePalette,
  greenPalette,
  orangePalette,
  blackPalette,
  deepTealPalette,
  redPalette,
  electricBluePalette,
  darkDeepForestPalette,
  darkGoldPalette,
  purplePalette,
} from './index';

export const THEME_LIST = [
  {
    label: '很经典的蓝色',
    value: 'blue',
    palette: bluePalette,
  },
  {
    label: '土豪金',
    value: 'darkGold',
    palette: darkGoldPalette,
  },
  {
    label: '像草原一样绿',
    value: 'green',
    palette: greenPalette,
  },
  {
    label: '基佬紫',
    value: 'purple',
    palette: purplePalette,
  },
  {
    label: '灰不拉几的蓝',
    value: 'deepTeal',
    palette: deepTealPalette,
  },
  {
    label: '果粒橙',
    value: 'orange',
    palette: orangePalette,
  },
  {
    label: '钛合金灰',
    value: 'black',
    palette: blackPalette,
  },
  {
    label: '小姑娘喜欢的粉红',
    value: 'red',
    palette: redPalette,
  },
  {
    label: '深墨绿',
    value: 'darkDeepForest',
    palette: darkDeepForestPalette,
  },
  {
    label: '电光蓝',
    value: 'electricBlue',
    palette: electricBluePalette,
  },
];

export const THEME_TO_PALETTE = THEME_LIST.reduce(
  (acc, item) => {
    acc[item.value] = {
      value: item.value,
      label: item.label,
      palette: item.palette,
    };
    return acc;
  },
  {} as Record<string, { value: string; label: string; palette: any }>,
);
