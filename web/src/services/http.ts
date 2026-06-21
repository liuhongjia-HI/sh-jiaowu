import axios from 'axios';
import type { ApiResponse, AuthResult } from '../types/starline';

const TOKEN_KEY = 'starline_admin_token';
const USER_KEY = 'starline_admin_user';
const DEFAULT_PRODUCTION_API_BASE_URL = 'https://gate.starlineeducation.com.cn/api';

function resolveApiBaseUrl() {
  return import.meta.env.VITE_API_BASE_URL || (import.meta.env.PROD ? DEFAULT_PRODUCTION_API_BASE_URL : '/api');
}

export const http = axios.create({
  baseURL: resolveApiBaseUrl(),
  timeout: 15000
});

http.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY);
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  const user = getSavedUser();
  if (user) {
    config.headers['X-Operator-ID'] = user.userId;
    config.headers['X-Operator-Name'] = encodeURIComponent(user.name);
  }
  return config;
});

http.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem(TOKEN_KEY);
      localStorage.removeItem(USER_KEY);
    }
    const message =
      error.response?.data?.message ||
      (error.response ? '操作失败，请检查后重试。' : '网络连接异常，请稍后重试。');
    const normalized = new Error(message);
    Object.assign(normalized, {
      response: error.response,
      status: error.response?.status
    });
    return Promise.reject(normalized);
  }
);

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function saveToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
}

function saveUser(user: AuthResult['user']) {
  localStorage.setItem(USER_KEY, JSON.stringify(user));
}

function getSavedUser(): AuthResult['user'] | null {
  const raw = localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as AuthResult['user'];
  } catch {
    localStorage.removeItem(USER_KEY);
    return null;
  }
}

export async function loginWithWechatCode(code: string) {
  const response = await http.post<ApiResponse<AuthResult>>('/auth/wechat-login', { code });
  saveToken(response.data.data.token);
  saveUser(response.data.data.user);
  return response.data.data;
}

export async function loginWithAdminPassword(phone: string, password: string) {
  const response = await http.post<ApiResponse<AuthResult>>('/auth/admin-password-login', { phone, password });
  saveToken(response.data.data.token);
  saveUser(response.data.data.user);
  return response.data.data;
}

export async function getData<T>(url: string, params?: Record<string, string>) {
  const response = await http.get<ApiResponse<T>>(url, { params });
  return response.data.data;
}

export async function postData<T>(url: string, body: unknown) {
  const response = await http.post<ApiResponse<T>>(url, body);
  return response.data.data;
}

export async function putData<T>(url: string, body: unknown) {
  const response = await http.put<ApiResponse<T>>(url, body);
  return response.data.data;
}

export async function postForm<T>(url: string, body: FormData) {
  const response = await http.post<ApiResponse<T>>(url, body, {
    headers: { 'Content-Type': 'multipart/form-data' }
  });
  return response.data.data;
}
