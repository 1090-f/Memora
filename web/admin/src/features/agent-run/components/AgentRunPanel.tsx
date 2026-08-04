import { Alert, Chip, Divider, List, ListItem, ListItemText, Stack, Typography } from '@mui/material';
import type { AgentRunViewState } from '../types';

export function AgentRunPanel({ state }: { state: AgentRunViewState }) {
  return (
    <Stack spacing={2} p={2} sx={{ overflow: 'auto', height: '100%' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="h6">Agent 运行</Typography>
        <Chip size="small" label={state.status} />
      </Stack>
      {state.router && (
        <Alert severity="info"><strong>{state.router.execution_mode}</strong><br />{state.router.reason_summary}</Alert>
      )}
      {state.error && <Alert severity="error">{state.error.message}</Alert>}
      <Divider />
      <Typography fontWeight={700}>计划与工具</Typography>
      <List dense disablePadding>
        {state.plan?.steps.map((step) => <ListItem key={step.step_no}><ListItemText primary={step.title} secondary={step.status} /></ListItem>)}
        {state.tools.map((tool) => <ListItem key={tool.tool_call_id}><ListItemText primary={tool.tool_name} secondary={tool.output_summary || tool.status} /></ListItem>)}
      </List>
      {!state.plan && state.tools.length === 0 && <Typography color="text.secondary">运行后将在这里显示可观察摘要。</Typography>}
    </Stack>
  );
}
