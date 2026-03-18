import { useState, useCallback } from 'react';

export type ToastType = 'error' | 'warning' | 'info' | 'success';

interface ToastState {
  show: boolean;
  message: string;
  type: ToastType;
}

interface UseToastReturn {
  toast: ToastState;
  showToast: (message: string, type?: ToastType) => void;
  hideToast: () => void;
}

export const useToast = (duration = 4000): UseToastReturn => {
  const [toast, setToast] = useState<ToastState>({
    show: false,
    message: '',
    type: 'error',
  });

  const showToast = useCallback((message: string, type: ToastType = 'error') => {
    setToast({ show: true, message, type });
    setTimeout(() => {
      setToast(prev => ({ ...prev, show: false }));
    }, duration);
  }, [duration]);

  const hideToast = useCallback(() => {
    setToast(prev => ({ ...prev, show: false }));
  }, []);

  return { toast, showToast, hideToast };
};
