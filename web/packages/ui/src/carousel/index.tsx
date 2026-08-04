import { memo, useRef, useCallback, useState, useEffect } from 'react';
import { styled, alpha, Tabs, Tab, Box, useTheme } from '@mui/material';
import { StyledTopicTitle, StyledTopicBox } from '../component/styledCommon';
import { Swiper, SwiperSlide } from 'swiper/react';
import { useFadeInText } from '../hooks/useGsapAnimation';
import { Swiper as SwiperType } from 'swiper';
import { gsap } from 'gsap';

import 'swiper/css';
import 'swiper/css/pagination';

import { Pagination, Autoplay } from 'swiper/modules';
import './index.css';

interface CarouselProps {
  mobile?: boolean;
  title: string;
  items: {
    id: string;
    title: string;
    url: string;
    desc: string;
  }[];
}

export const indicatorContainerStyle = {
  bottom: 0,
};

export const indicatorIconButtonStyle = {
  width: 6,
  borderRadius: 2,
  background: 'rgba(255, 255, 255, 0.20)',
  height: 6,
  cursor: 'pointer',
  opacity: 1,
};

export const activeIndicatorIconButtonStyle = {
  background: 'rgba(255, 255, 255, 1)',
};

const StyledSwiperSlideImg = styled('img')(({ theme }) => ({
  aspectRatio: '16 / 9',
  [theme.breakpoints.down('md')]: {
    width: 440,
  },
  [theme.breakpoints.up('md')]: {
    width: 880,
  },
  objectFit: 'cover',
  borderRadius: '10px',
}));

const StyledSwiperSlideDesc = styled('div')(({ theme }) => ({
  position: 'absolute',
  bottom: '24px',
  left: '50%',
  transform: 'translateX(-50%)',
  padding: theme.spacing(0.5, 1),
  fontSize: 14,
  fontWeight: 400,
  color: theme.palette.background.default,
  borderRadius: '6px',
  overflow: 'hidden',
  whiteSpace: 'nowrap',
  zIndex: 0,
  '&::before': {
    content: '""',
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: alpha(theme.palette.text.primary, 0.5),
    filter: 'blur(6px)',
    borderRadius: '12px',
    zIndex: -1,
  },
}));

// 样式化的 Tabs 容器 - 浅灰色背景，圆角，阴影
const StyledTabsContainer = styled(Box)(({ theme }) => ({
  maxWidth: '100%',
  display: 'inline-flex',
  borderRadius: '10px',
  padding: theme.spacing(0.5),
  boxShadow: `0px 5px 20px 0px ${alpha(theme.palette.text.primary, 0.06)}`,
  border: `1px solid ${alpha(theme.palette.text.primary, 0.1)}`,
  gap: theme.spacing(0.5),
  marginBottom: '-20px',
}));

// 样式化的 Tabs 组件 - 移除默认样式
const StyledTabs = styled(Tabs)(({ theme }) => ({
  minHeight: 'auto',
  '& .MuiTabs-indicator': {
    display: 'none',
  },
  '& .MuiTabs-flexContainer': {
    gap: theme.spacing(0.5),
  },
}));

// 样式化的 Tab 组件 - 白色背景，圆角，深灰色文字
const StyledTab = styled(Tab)(({ theme }) => ({
  minHeight: 'auto',
  padding: theme.spacing(1, 2),
  borderRadius: '6px',
  backgroundColor: theme.palette.background.default,
  color: theme.palette.text.secondary,
  fontSize: 14,
  fontWeight: 400,
  textTransform: 'none',
  transition: 'all 0.2s ease-in-out',
  '&:hover': {
    backgroundColor: alpha(theme.palette.primary.main, 0.08),
  },
  '&.Mui-selected': {
    backgroundColor: theme.palette.primary.main,
    color: theme.palette.primary.contrastText,
    fontWeight: 500,
  },
}));

const Carousel = ({ title, items }: CarouselProps) => {
  const theme = useTheme();
  // 添加标题淡入动画
  const titleRef = useFadeInText(0.2, 0.1);
  // 添加Swiper ref
  const swiperRef = useRef<SwiperType | null>(null);
  const [activeTab, setActiveTab] = useState<string>(items[0]?.id || '');
  // 存储所有描述元素的 ref
  const descRefs = useRef<(HTMLDivElement | null)[]>([]);
  // 存储动画时间线，用于清理
  const animationTimelines = useRef<gsap.core.Timeline[]>([]);

  // 导航函数
  const handlePrev = useCallback(() => {
    if (swiperRef.current) {
      swiperRef.current.slidePrev();
    }
  }, []);

  const handleNext = useCallback(() => {
    if (swiperRef.current) {
      swiperRef.current.slideNext();
    }
  }, []);

  // 触发从左到右的文字出现动画（逐字符显示，容器逐渐撑大）
  const animateTextFromLeft = useCallback(
    (index: number) => {
      const descElement = descRefs.current[index];
      if (!descElement) return;

      // 清理之前的动画
      animationTimelines.current.forEach(tl => tl.kill());
      animationTimelines.current = [];

      const originalText = descElement.textContent || '';
      if (!originalText) return;

      // 获取容器的 padding 值
      const computedStyle = window.getComputedStyle(descElement);
      const paddingLeft = parseFloat(computedStyle.paddingLeft) || 0;
      const paddingRight = parseFloat(computedStyle.paddingRight) || 0;
      const padding = paddingLeft + paddingRight;

      // 将文字分割成字符
      const chars = Array.from(originalText);
      const charElements: HTMLSpanElement[] = [];

      // 清空容器并创建字符元素（初始都隐藏）
      descElement.innerHTML = '';
      chars.forEach(char => {
        const span = document.createElement('span');
        span.textContent = char === ' ' ? '\u00A0' : char; // 空格用非断行空格
        span.style.opacity = '0';
        span.style.display = 'inline-block';
        descElement.appendChild(span);
        charElements.push(span);
      });

      // 创建一个隐藏的测量容器来准确测量每个字符的宽度
      const measureContainer = document.createElement('div');
      measureContainer.style.position = 'absolute';
      measureContainer.style.visibility = 'hidden';
      measureContainer.style.whiteSpace = 'nowrap';
      measureContainer.style.fontSize = computedStyle.fontSize;
      measureContainer.style.fontWeight = computedStyle.fontWeight;
      measureContainer.style.fontFamily = computedStyle.fontFamily;
      document.body.appendChild(measureContainer);

      // 测量每个字符的宽度
      const charWidths: number[] = [];
      charElements.forEach(span => {
        measureContainer.textContent = span.textContent;
        const charWidth = measureContainer.offsetWidth;
        charWidths.push(charWidth);
      });

      document.body.removeChild(measureContainer);

      // 设置容器初始状态（只有 padding，背景色透明）
      gsap.set(descElement, {
        width: padding,
        minWidth: padding,
      });

      // 创建动画时间线，延迟 0.5 秒开始
      const tl = gsap.timeline({ delay: 0.5 });
      let currentWidth = padding;

      // 背景色从透明逐渐加深（与第一个字符同时开始）
      tl.to(
        descElement,
        {
          duration: 0.4, // 背景色变化稍快一些，在文字显示过程中完成
          ease: 'power2.out',
        },
        0,
      );

      // 逐个显示字符，同时增加容器宽度
      // 第一个字符在延迟后立即开始显示（时间位置 0）
      charElements.forEach((span, i) => {
        const charWidth = charWidths[i];
        currentWidth += charWidth;

        // 同时显示字符和增加容器宽度
        // 第一个字符立即显示（i=0 时时间为 0），后续字符依次延迟
        tl.to(
          span,
          {
            opacity: 1,
            duration: 0.08,
            ease: 'none',
          },
          i * 0.08,
        );

        // 同时更新容器宽度
        tl.to(
          descElement,
          {
            width: currentWidth,
            duration: 0.08,
            ease: 'none',
          },
          i * 0.08,
        );
      });

      // 保存动画时间线
      animationTimelines.current.push(tl);
    },
    [theme],
  );

  // 监听 Swiper 切换，更新 activeTab 并触发动画
  const handleSlideChange = useCallback(
    (swiper: SwiperType) => {
      const activeIndex = swiper.activeIndex;
      // 在 centeredSlides 模式下，activeIndex 直接对应 items 数组的索引
      const activeItem = items[activeIndex];
      if (activeItem) {
        setActiveTab(activeItem.id);
        // 触发当前幻灯片的文字动画
        animateTextFromLeft(activeIndex);
      }
    },
    [items, animateTextFromLeft],
  );

  // 当 activeTab 改变时，切换对应的 Swiper 卡片
  const handleTabChange = useCallback(
    (value: string) => {
      setActiveTab(value);
      const targetIndex = items.findIndex(item => item.id === value);
      if (targetIndex !== -1 && swiperRef.current) {
        swiperRef.current.slideTo(targetIndex);
        // 触发切换后的文字动画
        setTimeout(() => {
          animateTextFromLeft(targetIndex);
        }, 300); // 等待切换动画完成
      }
    },
    [items, animateTextFromLeft],
  );

  // 初始加载时触发第一个幻灯片的动画
  useEffect(() => {
    if (items.length > 0 && descRefs.current[0]) {
      // 延迟执行，确保元素已经渲染
      const timer = setTimeout(() => {
        animateTextFromLeft(0);
      }, 100);
      return () => clearTimeout(timer);
    }
  }, [items.length, animateTextFromLeft]);

  // 组件卸载时清理所有动画
  useEffect(() => {
    return () => {
      animationTimelines.current.forEach(tl => tl.kill());
      animationTimelines.current = [];
    };
  }, []);

  // 使用事件委托的方式处理点击事件
  const handleSlideClick = useCallback(
    (swiper: SwiperType, event: MouseEvent | TouchEvent | PointerEvent) => {
      const target = event.target as HTMLElement;
      // 查找最近的swiper-slide元素
      const slideElement = target.closest('.swiper-slide');

      if (slideElement) {
        // 检查是否包含swiper-slide-prev类名
        if (slideElement.classList.contains('swiper-slide-prev')) {
          handlePrev();
        }
        // 检查是否包含swiper-slide-next类名
        else if (slideElement.classList.contains('swiper-slide-next')) {
          handleNext();
        }
      }
    },
    [handlePrev, handleNext],
  );

  return (
    <StyledTopicBox
      sx={{
        '.swiper-pagination-bullets': indicatorContainerStyle,
        '.swiper-pagination-bullet': indicatorIconButtonStyle,
        '.swiper-pagination-bullet-active': activeIndicatorIconButtonStyle,
        '.swiper-slide-prev': {
          cursor: 'pointer',
        },
        '.swiper-slide-next': {
          cursor: 'pointer',
        },
      }}
    >
      <StyledTopicTitle ref={titleRef} sx={{ color: 'text.primary' }}>
        {title}
      </StyledTopicTitle>
      {items.length > 0 && (
        <StyledTabsContainer>
          <StyledTabs
            value={activeTab}
            onChange={(_, value) => {
              handleTabChange(value as string);
            }}
            variant='scrollable'
            scrollButtons={false}
          >
            {items.map(item => (
              <StyledTab key={item.id} label={item.title} value={item.id} />
            ))}
          </StyledTabs>
        </StyledTabsContainer>
      )}
      <Swiper
        onSwiper={swiper => {
          swiperRef.current = swiper;
        }}
        onSlideChange={handleSlideChange}
        onClick={handleSlideClick}
        spaceBetween={50}
        centeredSlides={true}
        pagination={{
          clickable: true,
        }}
        autoplay={{
          delay: 5000,
          disableOnInteraction: false,
          pauseOnMouseEnter: true,
        }}
        modules={[Pagination, Autoplay]}
        className='mySwiper'
      >
        {items?.map((item, index) => (
          <SwiperSlide key={item.id} style={{ position: 'relative' }}>
            <StyledSwiperSlideImg src={item.url} alt={item.title} />
            {item.desc && (
              <StyledSwiperSlideDesc
                ref={el => {
                  descRefs.current[index] = el;
                }}
              >
                {item.desc}
              </StyledSwiperSlideDesc>
            )}
          </SwiperSlide>
        ))}
      </Swiper>
    </StyledTopicBox>
  );
};

export default memo(Carousel);
