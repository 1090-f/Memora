import { TrendData } from '@/api';
import { Box, useTheme } from '@mui/material';
import type { ECharts } from 'echarts';
import { useEffect, useRef, useState } from 'react';
import { loadScript, loadScriptsInOrder } from '@/utils/loadScript';

interface Props {
  map: 'china' | 'world' | string;
  data: TrendData[];
  tooltipText: string;
}

const MapChart = ({ map, data: chartData, tooltipText }: Props) => {
  const theme = useTheme();
  const domWrapRef = useRef<HTMLDivElement>(null);
  const echartRef = useRef<ECharts>(null!);
  const [max, setMax] = useState(0);
  const [data, setData] = useState<{ name: string; value: number }[]>([]);
  const [resourceLoaded, setResourceLoaded] = useState(false);

  useEffect(() => {
    let isUnmounted = false;

    const toAbsUrl = (pathname: string) =>
      new URL(pathname, window.location.origin).toString();

    const withBasenameCandidates = (pathname: string) => {
      const base = window.__BASENAME__ || '';
      const normalizedBase = base.endsWith('/') ? base.slice(0, -1) : base;
      return [
        toAbsUrl(`${normalizedBase}${pathname}`),
        toAbsUrl(pathname), // fallback: 资源挂在站点根路径
      ];
    };

    const loadScriptWithFallback = async (urls: string[]) => {
      let lastErr: unknown;
      for (const url of urls) {
        try {
          await loadScript(url);
          return;
        } catch (e) {
          lastErr = e;
        }
      }
      throw lastErr;
    };

    const load = async () => {
      try {
        await loadScriptWithFallback(
          withBasenameCandidates('/echarts/echarts.5.4.1.min.js'),
        );

        // 依赖 echarts 全局变量，必须顺序加载
        const chinaCandidates = withBasenameCandidates('/echarts/china.js');
        const geoCandidates = withBasenameCandidates('/geo/geo.js');
        await loadScriptsInOrder([chinaCandidates[0], geoCandidates[0]]).catch(
          async () => {
            // 如果 basename 版本 404，则回退到根路径版本
            await loadScriptsInOrder([chinaCandidates[1], geoCandidates[1]]);
          },
        );

        if (!isUnmounted) setResourceLoaded(true);
      } catch (e) {
        console.error('[MapChart] 资源加载失败', e);
      }
    };
    load();
    return () => {
      isUnmounted = true;
    };
  }, []);

  useEffect(() => {
    if (!resourceLoaded) return;
    setMax(Math.max(1, ...chartData.map(i => i.count)));
    setData(chartData.map(it => ({ name: it.name, value: it.count })));
    if (domWrapRef.current && !echartRef.current) {
      type EchartsGlobal = { init: (el: HTMLDivElement) => ECharts };
      const echartsGlobal = (window as unknown as { echarts: EchartsGlobal })
        .echarts;
      echartRef.current = echartsGlobal.init(domWrapRef.current);
    }
  }, [chartData, resourceLoaded]);

  useEffect(() => {
    if (!echartRef.current) return;
    const option = {
      grid: {
        top: 0,
        bottom: 0,
        right: 0,
        left: 0,
      },
      tooltip: {
        formatter: (params: { name: string; value: number | string }) => {
          return `${params.name}<br />${tooltipText}: <span style='font-weight: 700'>${params.value || 0}</span>`;
        },
      },
      visualMap: [
        {
          show: true,
          orient: 'horizontal',
          left: 8,
          bottom: 8,
          itemWidth: 10,
          color: ['#3082FF', '#EBF3FF'],
          max,
          textStyle: {
            color: theme.palette.primary.main,
          },
        },
      ],
      series: [
        {
          type: 'map',
          map,
          data: data,
          itemStyle: {
            borderColor: theme.palette.divider,
            areaColor: '#DDE4F0',
            emphasis: {
              show: true,
              areaColor: '#A9C0E3',
            },
          },
        },
      ],
    };

    echartRef.current.setOption(option, true);

    const resize = () => {
      if (echartRef.current) {
        echartRef.current.resize();
      }
    };
    window.addEventListener('resize', resize);
    return () => {
      window.removeEventListener('resize', resize);
    };
  }, [
    map,
    data,
    max,
    theme.palette.divider,
    theme.palette.primary.main,
    tooltipText,
  ]);

  // if (!loading) return <div style={{ width: '100%', height: 292 }} />
  return (
    <Box
      sx={{ width: '100%', height: 292, pr: '200px' }}
      ref={domWrapRef}
    ></Box>
  );
};

export default MapChart;
