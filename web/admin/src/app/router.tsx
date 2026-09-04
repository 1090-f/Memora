import { Navigate, useRoutes, type RouteObject } from 'react-router-dom';
import { RequireAuth } from './RequireAuth';
import { AnonymousLayout } from '@/layouts/AnonymousLayout';
import { AppShell } from '@/layouts/AppShell';
import { LoginPage } from '@/features/auth/pages/LoginPage';
import { KnowledgeBaseListPage } from '@/features/knowledge-base/pages/KnowledgeBaseListPage';
import { KnowledgeBaseSettingsPage } from '@/features/knowledge-base/pages/KnowledgeBaseSettingsPage';
import { DocumentWorkspacePage } from '@/features/document/pages/DocumentWorkspacePage';
import { ChatPage } from '@/features/conversation/pages/ChatPage';
import { AgentRunListPage } from '@/features/agent-run/pages/AgentRunListPage';
import { AgentRunDetailPage } from '@/features/agent-run/pages/AgentRunDetailPage';
import { MemoryPage } from '@/features/memory/pages/MemoryPage';
import { McpPage } from '@/features/mcp/pages/McpPage';
import { SearchTestPage } from '@/features/search/pages/SearchTestPage';
import { ProfilePage } from '@/features/user/pages/ProfilePage';
import { ModelSettingsPage } from '@/features/model/pages/ModelSettingsPage';

const routeObjects: RouteObject[] = [
  {
    element: <AnonymousLayout />,
    children: [{ path: '/login', element: <LoginPage /> }],
  },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <AppShell />,
        children: [
          { path: '/knowledge-bases', element: <KnowledgeBaseListPage /> },
          { path: '/kb/:kbId/docs/:documentId?', element: <DocumentWorkspacePage /> },
          { path: '/chat', element: <Navigate to="/knowledge-bases" replace /> },
          { path: '/chat/:kbId/:conversationId?', element: <ChatPage /> },
          { path: '/runs', element: <AgentRunListPage /> },
          { path: '/runs/:runId', element: <AgentRunDetailPage /> },
          { path: '/memories', element: <MemoryPage /> },
          { path: '/mcp', element: <McpPage /> },
          { path: '/kb/:kbId/search-test', element: <SearchTestPage /> },
          { path: '/kb/:kbId/settings', element: <KnowledgeBaseSettingsPage /> },
          { path: '/settings/profile', element: <ProfilePage /> },
          { path: '/settings/models', element: <ModelSettingsPage /> },
        ],
      },
    ],
  },
  { path: '/', element: <Navigate to="/knowledge-bases" replace /> },
  { path: '*', element: <Navigate to="/knowledge-bases" replace /> },
];

export function AppRoutes() {
  return useRoutes(routeObjects);
}
