import '@/assets/fonts/font.css';
import '@/assets/styles/index.css';
import '@/assets/styles/markdown.css';
import { wrapWindowOpen } from './utils/getBasename';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import duration from 'dayjs/plugin/duration';
import relativeTime from 'dayjs/plugin/relativeTime';
import { createRoot } from 'react-dom/client';
import { Provider } from 'react-redux';
import { BrowserRouter } from 'react-router-dom';
import App from './App';
import store from './store';

// 动态加载 CSS 文件
const loadCSS = (href: string) => {
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = href;
  document.head.appendChild(link);
};

loadCSS(`${window.__BASENAME__}/panda-wiki.css`);

wrapWindowOpen(window.__BASENAME__ || '');
dayjs.extend(duration);
dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

createRoot(document.getElementById('root')!).render(
  <BrowserRouter basename={window.__BASENAME__}>
    <Provider store={store}>
      <App />
    </Provider>
  </BrowserRouter>,
);
