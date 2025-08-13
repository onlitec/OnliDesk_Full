// Remote Access Components
export { default as RemoteAccessDashboard } from './RemoteAccessDashboard';
export { default as RemoteAccessPortal } from './RemoteAccessPortal';
export { default as SessionManager } from './SessionManager';
export { default as PrivilegeEscalation } from './PrivilegeEscalation';

// Services and Hooks
export { RemoteAccessService } from '../../services/RemoteAccessService';
export { useRemoteAccess } from '../../hooks/useRemoteAccess';

// Component exports for easier importing
export const RemoteAccessComponents = {
  Dashboard: RemoteAccessDashboard,
  Portal: RemoteAccessPortal,
  SessionManager,
  PrivilegeEscalation,
};

// Service and Hook exports
export const RemoteAccessUtils = {
  Service: RemoteAccessService,
  Hook: useRemoteAccess,
};

// Default export - main dashboard component
export default RemoteAccessDashboard;