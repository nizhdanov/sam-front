import { apicraft } from '@siberiacancode/apicraft';

export default apicraft([
  {
    input: '../back/api/openapi.yaml',
    output: 'generated/api',
    instance: 'axios',
    nameBy: 'path',
    groupBy: 'tags',
    baseUrl: '/api/v1'
  }
]);
