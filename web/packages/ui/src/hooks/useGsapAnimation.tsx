import { useEffect, useRef, useState } from 'react';
import { gsap } from 'gsap';

// 文字渐入动画 hook
export const useTextAnimation = (
  delay: number = 0,
  threshold: number = 0.1,
) => {
  const textRef = useRef<HTMLHeadingElement>(null);
  const [isVisible, setIsVisible] = useState(false);
  const [hasAnimated, setHasAnimated] = useState(false);

  useEffect(() => {
    if (!textRef.current || hasAnimated) return;

    const text = textRef.current;

    // 创建 Intersection Observer
    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (entry.isIntersecting && !hasAnimated) {
            setIsVisible(true);
            setHasAnimated(true);
          }
        });
      },
      {
        threshold,
        rootMargin: '0px 0px -50px 0px', // 提前 50px 触发
      },
    );

    observer.observe(text);

    return () => {
      observer.disconnect();
    };
  }, [threshold, hasAnimated]);

  useEffect(() => {
    if (!textRef.current || !isVisible) return;

    const text = textRef.current;

    // 创建文字分割效果 - 支持表情符号
    const splitText = (element: HTMLElement) => {
      const text = element.textContent || '';
      const chars = Array.from(text).map(char => {
        return char === ' ' ? '&nbsp;' : char;
      });

      element.innerHTML = chars
        .map(
          char =>
            `<span style="display: inline-block; opacity: 0; transform: translateY(20px);">${char}</span>`,
        )
        .join('');

      return element.querySelectorAll('span');
    };

    // 分割文字
    const chars = splitText(text);

    // 设置初始状态
    gsap.set(chars, {
      opacity: 0,
      y: 20,
      rotationX: -90,
    });

    // 根据文字长度动态调整动画参数
    const textLength = chars.length;
    const duration = Math.min(
      0.5,
      Math.max(0.2, 0.5 - (textLength - 10) * 0.015),
    ); // 长文本减少单字符时间
    const stagger = Math.min(
      0.03,
      Math.max(0.01, 0.03 - (textLength - 20) * 0.0008),
    ); // 长文本减少间隔时间
    const easeStrength = Math.min(
      1.4,
      Math.max(1.0, 1.4 - (textLength - 15) * 0.015),
    ); // 长文本减少回弹强度

    // 创建动画时间线
    const tl = gsap.timeline({ delay });

    // 逐个字符动画
    tl.to(chars, {
      opacity: 1,
      y: 0,
      rotationX: 0,
      duration,
      stagger,
      ease: `back.out(${easeStrength})`,
    });

    // 清理函数
    return () => {
      tl.kill();
    };
  }, [isVisible, delay]);

  return textRef;
};

// 文字淡入动画 hook（更简单的版本）
export const useFadeInText = (delay: number = 0, threshold: number = 0.1) => {
  const textRef = useRef<HTMLDivElement>(null);
  const [isVisible, setIsVisible] = useState(false);
  const [hasAnimated, setHasAnimated] = useState(false);

  useEffect(() => {
    if (!textRef.current || hasAnimated) return;

    const text = textRef.current;

    // 创建 Intersection Observer
    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (entry.isIntersecting && !hasAnimated) {
            setIsVisible(true);
            setHasAnimated(true);
          }
        });
      },
      {
        threshold,
        rootMargin: '0px 0px -50px 0px',
      },
    );

    observer.observe(text);

    return () => {
      observer.disconnect();
    };
  }, [threshold, hasAnimated]);

  useEffect(() => {
    if (!textRef.current || !isVisible) return;

    const text = textRef.current;

    // 设置初始状态
    gsap.set(text, {
      opacity: 0,
      y: 30,
    });

    // 创建动画
    const tl = gsap.timeline({ delay });

    tl.to(text, {
      opacity: 1,
      y: 0,
      duration: 0.6,
      ease: 'power2.out',
    });

    return () => {
      tl.kill();
    };
  }, [isVisible, delay]);

  return textRef;
};

// 文字打字机效果 hook
export const useTypewriterText = (
  delay: number = 0,
  speed: number = 0.05,
  threshold: number = 0.1,
) => {
  const textRef = useRef<HTMLElement>(null);
  const [isVisible, setIsVisible] = useState(false);
  const [hasAnimated, setHasAnimated] = useState(false);

  useEffect(() => {
    if (!textRef.current || hasAnimated) return;

    const text = textRef.current;

    // 创建 Intersection Observer
    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (entry.isIntersecting && !hasAnimated) {
            setIsVisible(true);
            setHasAnimated(true);
          }
        });
      },
      {
        threshold,
        rootMargin: '0px 0px -50px 0px',
      },
    );

    observer.observe(text);

    return () => {
      observer.disconnect();
    };
  }, [threshold, hasAnimated]);

  useEffect(() => {
    if (!textRef.current || !isVisible) return;

    const text = textRef.current;
    const originalText = text.textContent || '';

    // 清空文字
    text.textContent = '';

    // 创建光标元素
    const cursor = document.createElement('span');
    cursor.textContent = '|';
    cursor.style.opacity = '1';
    cursor.style.animation = 'blink 1s infinite';

    // 添加光标样式
    const style = document.createElement('style');
    style.textContent = `
      @keyframes blink {
        0%, 50% { opacity: 1; }
        51%, 100% { opacity: 0; }
      }
    `;
    document.head.appendChild(style);

    text.appendChild(cursor);

    // 打字机动画
    const tl = gsap.timeline({ delay });

    for (let i = 0; i <= originalText.length; i++) {
      tl.to(text, {
        duration: speed,
        onUpdate: () => {
          text.textContent = originalText.slice(0, i);
          text.appendChild(cursor);
        },
      });
    }

    // 移除光标
    tl.call(() => {
      cursor.remove();
    });

    return () => {
      tl.kill();
      style.remove();
    };
  }, [isVisible, delay, speed]);

  return textRef;
};

// 卡片渐入动画 hook
export const useCardFadeInAnimation = (
  delay: number = 0,
  threshold: number = 0.1,
) => {
  const cardRef = useRef<HTMLElement>(null);
  const [isVisible, setIsVisible] = useState(false);
  const [hasAnimated, setHasAnimated] = useState(false);

  useEffect(() => {
    if (!cardRef.current || hasAnimated) return;

    const card = cardRef.current;

    // 创建 Intersection Observer
    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (entry.isIntersecting && !hasAnimated) {
            setIsVisible(true);
            setHasAnimated(true);
          }
        });
      },
      {
        threshold,
        rootMargin: '0px 0px -50px 0px',
      },
    );

    observer.observe(card);

    return () => {
      observer.disconnect();
    };
  }, [threshold, hasAnimated]);

  useEffect(() => {
    if (!cardRef.current || !isVisible) return;

    const card = cardRef.current;

    // 设置初始状态
    gsap.set(card, {
      opacity: 0,
      y: 50,
    });

    // 创建动画
    const tl = gsap.timeline({ delay });

    tl.to(card, {
      opacity: 1,
      y: 0,
      duration: 0.4,
      ease: 'back.out(1.4)',
    });

    return () => {
      tl.kill();
    };
  }, [isVisible, delay]);

  return cardRef;
};

export const useCardScaleAnimation = ({
  delay = 0,
  threshold = 0.1,
  duration = 0.4,
}: {
  delay?: number;
  threshold?: number;
  duration?: number;
}) => {
  const cardRef = useRef<HTMLElement>(null);
  const [isVisible, setIsVisible] = useState(false);
  const [hasAnimated, setHasAnimated] = useState(false);

  useEffect(() => {
    if (!cardRef.current || hasAnimated) return;

    const card = cardRef.current;

    // 创建 Intersection Observer
    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (entry.isIntersecting && !hasAnimated) {
            setIsVisible(true);
            setHasAnimated(true);
          }
        });
      },
      {
        threshold,
        rootMargin: '0px 0px -50px 0px',
      },
    );

    observer.observe(card);

    return () => {
      observer.disconnect();
    };
  }, [threshold, hasAnimated]);

  useEffect(() => {
    if (!cardRef.current || !isVisible) return;

    const card = cardRef.current;

    // 设置初始状态
    gsap.set(card, {
      opacity: 0,
      scale: 0,
    });

    // 创建动画
    const tl = gsap.timeline({ delay });

    tl.to(card, {
      opacity: 1,
      scale: 1,
      duration,
    });

    return () => {
      tl.kill();
    };
  }, [isVisible, delay]);

  return cardRef;
};

export const useCardAnimation = ({
  delay = 0,
  threshold = 0.1,
  initial,
  to,
}: {
  delay?: number;
  threshold?: number;
  initial: GSAPTweenVars;
  to: GSAPTweenVars;
}) => {
  const cardRef = useRef<HTMLElement>(null);
  const [isVisible, setIsVisible] = useState(false);
  const [hasAnimated, setHasAnimated] = useState(false);

  useEffect(() => {
    if (!cardRef.current || hasAnimated) return;

    const card = cardRef.current;

    // 创建 Intersection Observer
    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (entry.isIntersecting && !hasAnimated) {
            setIsVisible(true);
            setHasAnimated(true);
          }
        });
      },
      {
        threshold,
        rootMargin: '0px 0px -50px 0px',
      },
    );

    observer.observe(card);

    return () => {
      observer.disconnect();
    };
  }, [threshold, hasAnimated]);

  useEffect(() => {
    if (!cardRef.current || !isVisible) return;

    const card = cardRef.current;

    // 设置初始状态
    gsap.set(card, initial);

    // 创建动画
    const tl = gsap.timeline({ delay });

    tl.to(card, to);

    return () => {
      tl.kill();
    };
  }, [isVisible, delay]);

  return cardRef;
};
