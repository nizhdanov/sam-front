import { QueryClient } from '@tanstack/react-query';

import { ApiError } from '@/utils/api/errors';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (failureCount, error) =>
        failureCount < 2 &&
        !(error instanceof ApiError && error.status !== undefined && error.status < 500)
    }
  }
});
