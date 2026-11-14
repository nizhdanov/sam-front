import axios from 'axios';

export const instance = axios.create({ baseURL: '/api/v1' });

instance.interceptors.request.use((config) => {
  config.headers.Authorization = `Bearer ${localStorage.getItem('token')}`;

  return config;
});
