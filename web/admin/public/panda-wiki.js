(function () {
  let hasInitialized = false;

  const showPandaWiki = localStorage.getItem('show-panda-wiki') || '';
  const positionStorage = localStorage.getItem('panda-wiki-position') || '';
  const [left, top] = positionStorage.split(',');

  const script = document.currentScript.src;
  const origin = new URL(script).origin;
  const link = new URL(script).searchParams.get('link');
  const tools = new URL(script).searchParams.get('tools');

  const makeDraggable = (element, icon) => {
    let isDragging = false;
    let startX, startY, initialX, initialY;
    let animationFrameId = null;
    let dragTimer = null;

    const onMouseDown = (e) => {
      startX = e.clientX;
      startY = e.clientY;
      const rect = element.getBoundingClientRect();
      initialX = rect.left;
      initialY = rect.top;

      // 设置0.5秒的定时器，延迟设置拖拽状态
      dragTimer = setTimeout(() => {
        isDragging = true;
        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', onMouseUp);
      }, 500);

      document.addEventListener('mouseup', onMouseUp);
    };

    const onMouseMove = (e) => {
      if (!isDragging) return;
      const dx = e.clientX - startX;
      const dy = e.clientY - startY;
      if (animationFrameId) {
        cancelAnimationFrame(animationFrameId);
      }
      animationFrameId = requestAnimationFrame(() => {
        element.style.left = `${initialX + dx}px`;
        element.style.top = `${initialY + dy}px`;
        localStorage.setItem('panda-wiki-position', `${initialX + dx}px,${initialY + dy}px`);
      });
    };

    const onMouseUp = () => {
      // 清除定时器
      if (dragTimer) {
        clearTimeout(dragTimer);
        dragTimer = null;
      }

      // 如果没有进入拖拽状态，则不执行任何操作
      if (!isDragging) {
        document.removeEventListener('mouseup', onMouseUp);
        return;
      }

      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
      if (animationFrameId) {
        cancelAnimationFrame(animationFrameId);
        animationFrameId = null;
      }
    };

    icon.addEventListener('click', (e) => {
      if (isDragging) {
        e.stopPropagation();
      } else {
        isDragging = false;
      }
    });

    icon.addEventListener('mousedown', onMouseDown);
  };

  const createWidget = (element) => {
    const widget = document.createElement('div');
    widget.className = 'panda-wiki-widget';

    const search_text = document.createElement('div');
    search_text.className = 'panda-wiki-search';
    search_text.innerHTML = '开始搜索您的问题';
    widget.appendChild(search_text);
    element.appendChild(widget);

    const ai_text = document.createElement('div');
    ai_text.className = 'panda-wiki-text';
    ai_text.innerHTML = 'AI 小助手';
    element.appendChild(ai_text);
  }

  const createLogo = (element) => {
    const icon = document.createElement('div');
    icon.className = 'panda-wiki-icon';
    icon.innerHTML = `<div>
<svg class="panda-wiki-icon-panda" width="60px" height="60px" viewBox="0 0 28 28">
    <title>单独logo备份 3</title>
    <defs>
        <rect id="path-1" x="0" y="0" width="26.6666667" height="26.6666667"></rect>
    </defs>
    <g id="panda-wiki" stroke="none" stroke-width="1" fill="none" fill-rule="evenodd">
        <g id="单独logo备份-3" transform="translate(0.666667, 0.666667)">
            <mask id="mask-2" fill="white">
                <use xlink:href="#path-1"></use>
            </mask>
            <g id="蒙版"></g>
            <g id="编组-2" mask="url(#mask-2)">
                <g transform="translate(1.364432, -4.083715)" id="太极-copy">
                    <g transform="translate(10.774790, 9.658188) rotate(-30.000000) translate(-10.774790, -9.658188) translate(1.770494, 3.704506)">
                        <path d="M10.6296447,8.4877787 C10.0915528,7.29923003 11.5780278,7.41748585 12.215931,7.89475124 C12.8538342,8.37201663 13.7248035,9.34471545 13.7248035,10.754321 C13.7248035,12.1639265 11.9185394,12.3026988 11.4580211,11.0512699 C11.177605,10.289257 11.1677366,9.67632737 10.6296447,8.4877787 Z M12.1500714,5.08361583 C13.263065,4.13834064 14.8931355,3.94315846 16.2302206,4.71512487 C17.9308166,5.69696442 18.5134838,7.87150809 17.5316442,9.57210408 C17.1465888,10.2390397 16.5780915,10.7340342 15.9266819,11.0301509 C14.9170848,11.4890919 15.4943064,9.48732988 14.5389597,7.84427688 C13.5964546,6.22330938 11.8474838,5.34060614 12.1500714,5.08361583 Z M8.4193934,9.51982081 C8.88351147,10.2049803 7.60144814,10.7765698 6.88457493,10.4869342 C6.16770172,10.1972986 5.69002162,8.79591536 6.40045726,8.70411766 C7.1108929,8.61231996 7.95527532,8.83466131 8.4193934,9.51982081 Z M5.44443593,6.49441337 C4.38416032,7.25468075 4.06914921,7.7805592 3.44529531,8.30027363 C2.42076036,9.15378371 0.925875843,8.13047138 1.63067862,6.90971717 C2.33548139,5.68896296 3.57611239,5.28206575 4.36718545,5.18769339 C5.15825851,5.09332103 6.50471154,5.73414599 5.44443593,6.49441337 Z M0.47694781,1.77837147 C1.45878736,0.077775473 3.63333103,-0.504891746 5.33392702,0.47694781 C6.32821492,1.0510002 6.94035082,2.03276573 7.08108017,3.08914193 C7.13042355,3.45953493 5.11364883,3.26295291 3.46738177,3.72845398 C1.77832386,4.20605467 0.456176298,5.33696004 0.307044857,5.00081907 C-0.138195462,3.99725128 -0.114021143,2.80195972 0.47694781,1.77837147 Z" id="形状结合" fill="#ffffff" fill-rule="nonzero"></path>
                    </g>
                </g>
            </g>
        </g>
    </g>
</svg>
<svg class="panda-wiki-icon-taiji" width="60px" height="60px" viewBox="0 0 28 28">
    <title>单独logo</title>
    <defs>
        <rect id="path-1" x="0" y="0" width="26.6666667" height="26.6666667"></rect>
    </defs>
    <g id="panda-wiki" stroke="none" stroke-width="1" fill="none" fill-rule="evenodd">
        <g id="单独logo" transform="translate(0.666667, 0.666667)">
            <mask id="mask-2" fill="white">
                <use xlink:href="#path-1"></use>
            </mask>
            <g id="蒙版"></g>
            <g id="编组-2" mask="url(#mask-2)">
                <g transform="translate(-1.237604, -1.237604)" id="太极-copy">
                    <g transform="translate(14.570938, 14.570938) rotate(-30.000000) translate(-14.570938, -14.570938) translate(3.904271, 3.904271)">
                        <path d="M10.6674922,2.08997124e-13 C4.78431073,2.08997124e-13 -2.31587209e-13,4.78375517 -2.31587209e-13,10.6662487 C-2.31587209e-13,16.5478645 4.78431073,21.3333333 10.6674922,21.3333333 C16.5489808,21.3333333 21.3333333,16.5478645 21.3333333,10.6662487 C21.3333333,4.78375517 16.5490017,2.08997124e-13 10.6674922,2.08997124e-13 M10.6674922,1.20270442 C15.8947831,1.20270442 20.1330338,5.44046539 20.1330338,10.6662487 C20.1330338,11.3136804 20.0671145,11.9467135 19.9420058,12.5577831 C19.3317194,14.48313 17.5312284,15.8759408 15.4028128,15.8759408 C12.7739834,15.8759408 10.6387544,13.7452322 10.6387544,11.1142118 C10.6387544,8.48484231 8.5086251,6.35242012 5.87643073,6.35242012 C3.58566232,6.35242012 1.67531968,7.96923336 1.21885892,10.1219054 C1.49952795,5.14971569 5.62282748,1.20270442 10.6674922,1.20270442" id="形状" fill="#ffffff" fill-rule="nonzero"></path>
                    </g>
                </g>
            </g>
        </g>
    </g>
</svg>
    </div>`;
    element.appendChild(icon);
    makeDraggable(element, icon);
  }

  const createHideModal = (element) => {
    const hideModal = document.createElement('div');
    hideModal.className = 'panda-wiki-hide-modal';

    const hideContainer = document.createElement('div');
    hideContainer.className = 'panda-wiki-hide-container';
    hideContainer.innerHTML = `<div class="panda-wiki-hide-content">
    <div class="panda-wiki-hide-header">
      <svg viewBox="0 0 1024 1024" p-id="6652" width="20" height="20"><path d="M547.13616094 547.13616094H476.86383906V301.0625h70.27232188v246.07366095z m0 175.80133906H476.86383906V652.60491094h70.27232188V722.9375zM512 90.125a421.875 421.875 0 1 0 0 843.75A421.875 421.875 0 0 0 512 90.125z" p-id="6653" fill="#FEA145"></path></svg>
      隐藏挂件
    </div>
  </div>`;

    const hideBody = document.createElement('div');
    hideBody.className = 'panda-wiki-hide-body';

    const option1 = document.createElement('div');
    option1.className = 'panda-wiki-hide-option';

    const radio1 = document.createElement('input');
    radio1.type = 'radio';
    radio1.name = 'panda-wiki-hide-radio';
    radio1.id = 'panda-wiki-hide-radio-one';
    radio1.value = 'one';
    radio1.checked = true;
    option1.appendChild(radio1);

    const label1 = document.createElement('label');
    label1.htmlFor = 'panda-wiki-hide-radio-one';
    label1.innerHTML = '隐藏本次 <span>将在下次刷新页面时展示并复位挂件</span>';
    option1.appendChild(label1);

    hideBody.appendChild(option1);

    const option2 = document.createElement('div');
    option2.className = 'panda-wiki-hide-option';

    const radio2 = document.createElement('input');
    radio2.type = 'radio';
    radio2.name = 'panda-wiki-hide-radio';
    radio2.value = 'one-week';
    radio2.id = 'panda-wiki-hide-radio-one-week';
    option2.appendChild(radio2);

    const label2 = document.createElement('label');
    label2.htmlFor = 'panda-wiki-hide-radio-one-week';
    label2.innerHTML = '隐藏 7 天 <span>7 天后展示并复位挂件</span>';
    option2.appendChild(label2);

    hideBody.appendChild(option2);
    hideContainer.appendChild(hideBody);

    const closeIconBtn = document.createElement('div');
    closeIconBtn.className = 'panda-wiki-hide-modal-icon';
    closeIconBtn.innerHTML = '<svg viewBox="0 0 1024 1024" p-id="3836" width="16" height="16"><path d="M758.848 731.456c12.16-12.224 12.16-32 0-44.16L583.616 512l175.232-175.232c12.16-12.16 12.16-32 0-44.16l-27.392-27.456a31.232 31.232 0 0 0-44.16 0L512 440.384 336.768 265.152a31.232 31.232 0 0 0-44.16 0l-27.456 27.392c-12.16 12.224-12.16 32 0 44.16L440.384 512l-175.232 175.232c-12.16 12.16-12.16 32 0 44.16l27.392 27.456c12.224 12.16 32 12.16 44.16 0L512 583.616l175.232 175.232c12.16 12.16 32 12.16 44.16 0l27.456-27.392z" p-id="3837" fill="#21222D"></path></svg>'
    hideContainer.appendChild(closeIconBtn);

    closeIconBtn.addEventListener('click', () => {
      hideModal.classList.remove('active');
    })

    const hideFooter = document.createElement('div');
    hideFooter.className = 'panda-wiki-hide-footer';
    hideContainer.appendChild(hideFooter);

    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'panda-wiki-hide-cancel-btn';
    cancelBtn.innerHTML = '取消';
    hideFooter.appendChild(cancelBtn);

    const confirmBtn = document.createElement('button');
    confirmBtn.className = 'panda-wiki-hide-confirm-btn';
    confirmBtn.innerHTML = '确认';
    hideFooter.appendChild(confirmBtn);

    hideModal.appendChild(hideContainer);
    document.body.appendChild(hideModal);

    cancelBtn.addEventListener('click', () => {
      hideModal.classList.remove('active');
    })

    confirmBtn.addEventListener('click', () => {
      const selectedOption = document.querySelector('input[name="panda-wiki-hide-radio"]:checked').value
      if (selectedOption === 'one-week') {
        localStorage.setItem('show-panda-wiki', Date.now() + 7 * 24 * 60 * 60 * 1000);
      }
      localStorage.removeItem('panda-wiki-position');
      hideModal.classList.remove('active');
      element.style.display = 'none';
    })

    hideModal.addEventListener('click', (e) => {
      if (e.target === hideModal) {
        hideModal.classList.remove('active');
      }
    });

    const closeIcon = document.createElement('div');
    closeIcon.className = 'panda-wiki-hide-btn';
    closeIcon.innerHTML = '<svg viewBox="0 0 1024 1024" p-id="6330" id="mx_n_1743146027742" width="16" height="16"><path d="M758.848 731.456c12.16-12.224 12.16-32 0-44.16L583.616 512l175.232-175.232c12.16-12.16 12.16-32 0-44.16l-27.392-27.456a31.232 31.232 0 0 0-44.16 0L512 440.384 336.768 265.152a31.232 31.232 0 0 0-44.16 0l-27.456 27.392c-12.16 12.224-12.16 32 0 44.16L440.384 512l-175.232 175.232c-12.16 12.16-12.16 32 0 44.16l27.392 27.456c12.224 12.16 32 12.16 44.16 0L512 583.616l175.232 175.232c12.16 12.16 32 12.16 44.16 0l27.456-27.392z" p-id="6331" fill="#909095"></path></svg>'
    element.appendChild(closeIcon);

    closeIcon.addEventListener('click', (event) => {
      event.stopPropagation();
      hideModal.classList.add('active');
    })
  }

  const createIframe = (element) => {
    const modal = document.createElement('div');
    modal.className = 'panda-wiki-modal';
    const iframeContainer = document.createElement('div');
    iframeContainer.className = 'panda-wiki-iframe-container';
    const closeBtn = document.createElement('div');
    closeBtn.className = 'panda-wiki-modal-close';
    closeBtn.innerHTML = '<svg viewBox="0 0 1024 1024" p-id="3836" width="16" height="16"><path d="M758.848 731.456c12.16-12.224 12.16-32 0-44.16L583.616 512l175.232-175.232c12.16-12.16 12.16-32 0-44.16l-27.392-27.456a31.232 31.232 0 0 0-44.16 0L512 440.384 336.768 265.152a31.232 31.232 0 0 0-44.16 0l-27.456 27.392c-12.16 12.224-12.16 32 0 44.16L440.384 512l-175.232 175.232c-12.16 12.16-12.16 32 0 44.16l27.392 27.456c12.224 12.16 32 12.16 44.16 0L512 583.616l175.232 175.232c12.16 12.16 32 12.16 44.16 0l27.456-27.392z" p-id="3837" fill="#21222D"></path></svg>'
    iframeContainer.appendChild(closeBtn);
    const iframe = document.createElement('iframe');
    iframe.className = 'panda-wiki-iframe';
    iframe.src = `${origin}/plugin/${link}?tools=${tools}`
    element.addEventListener('click', () => {
      iframeContainer.appendChild(iframe);
      modal.classList.add('active');
    });
    closeBtn.addEventListener('click', () => {
      iframeContainer.removeChild(iframe);
      modal.classList.remove('active');
    });
    modal.addEventListener('click', (e) => {
      if (e.target === modal) {
        modal.classList.remove('active');
      }
    });
    modal.appendChild(iframeContainer);
    document.body.appendChild(modal);
  }

  const init = () => {
    if (hasInitialized) return;
    hasInitialized = true;

    const container = document.createElement('div');
    container.className = 'panda-wiki-container';

    if (showPandaWiki && Date.now() < showPandaWiki) {
      return
    }

    if (link) {
      fetch(`${origin}/share/v1/app/link?link=${link}`).then(res => {
        if (res.ok) {
          res.json().then(data => {
            const position = data?.data?.settings?.position || [4, 24, 24];
            switch (position[0]) {
              case 1:
                container.style.top = position[1] + 'px'
                container.style.left = position[2] + 'px'
                break;
              case 2:
                container.style.top = position[1] + 'px'
                container.style.right = position[2] + 'px'
                break;
              case 3:
                container.style.bottom = position[1] + 'px'
                container.style.left = position[2] + 'px'
                break;
              case 5:
                container.style.top = 'calc(50% - 34px)'
                container.style.left = position[2] + 'px'
                break;
              case 6:
                container.style.top = 'calc(50% - 34px)'
                container.style.right = position[2] + 'px'
                break;
              default:
                container.style.bottom = position[1] + 'px'
                container.style.right = position[2] + 'px'
            }
            if (positionStorage) {
              container.style.left = left
              container.style.top = top
            }
            container.style.display = 'block';
          })
        }
      })
    }
    createWidget(container);
    createLogo(container);
    createHideModal(container);
    createIframe(container);
    document.body.appendChild(container);
  }

  if (document.readyState === 'complete') init();
  else if (document.readyState === 'interactive') {
    document.addEventListener('DOMContentLoaded', init, { once: true });
  } else {
    document.addEventListener('DOMContentLoaded', init, { once: true });
  }
  window.addEventListener('load', init, { once: true });
})();
