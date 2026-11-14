import { apicraft } from '@siberiacancode/apicraft';

export default apicraft([
  {
    input: 'https://sam.v1sdev.ru/api/v1/docs/openapi.yaml',
    output: 'generated/api',
    instance: {
      name: 'axios',
      runtimeInstancePath: './src/utils/lib/instance'
    },
    nameBy: 'path',
    groupBy: 'tags',
    baseUrl: '/api/v1'
  }
]);
