<template>
  <div class="flex min-h-screen items-center justify-center bg-[var(--memora-bg)]">
    <div class="text-center">
      <svg
        class="mx-auto mb-4 h-16 w-16 text-[var(--memora-muted)]"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
        />
      </svg>
      <h2 class="mb-2 text-lg font-semibold text-[var(--memora-text)]">
        需要桌面端访问
      </h2>
      <p class="text-sm text-[var(--memora-muted)]">
        Memora P0 需要至少 1280px 宽度的桌面浏览器窗口。
      </p>
      <p class="mt-1 text-sm text-[var(--memora-muted)]">
        当前窗口宽度不足，请调整浏览器窗口大小。
      </p>
    </div>
  </div>
</template>
