import { apicraft } from '@siberiacancode/apicraft';

export default apicraft([
  {
    input: 'https://sam.v1sdev.ru/api/v1/docs/openapi.yaml',
    output: 'generated/api',
    instance: 'axios',
    nameBy: 'path',
    groupBy: 'tags',
    baseUrl: '/api/v1'
  }
]);
