import axios from 'axios';
import type { ApiResponse, AuthResult, CaptchaChallenge, PasswordResetResult } from '../types/starline';

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

// 后端返回的图片地址（轮播图、头像）是相对服务器根路径的绝对路径（如 /api/banners/images/xxx），
// 已经带着 /api 前缀了。本地开发时 baseURL 是同源的 '/api'，走 vite 代理直接能用；
// 生产环境 baseURL 是跨域的完整网关地址，<img> 标签不会经过 axios 的 baseURL 拼接，
// 必须自己把网关源拼回去，否则图片会被当成当前后台域名下的路径去请求，404。
export function resolveAssetUrl(path?: string) {
  if (!path) return '';
  if (/^https?:\/\//i.test(path)) return path;
  const base = resolveApiBaseUrl();
  if (!/^https?:\/\//i.test(base)) return path;
  const origin = base.replace(/\/api\/?$/, '');
  return `${origin}${path}`;
}

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

// 文件类接口用 responseType: 'blob' 请求，出错时 response.data 是 Blob 而不是对象，
// 直接读 data.message 永远拿不到后端文案，报错会退化成通用的“操作失败”。
async function readErrorMessage(error: { response?: { data?: unknown } }) {
  if (!error.response) return '网络连接异常，请稍后重试。';
  let data: unknown = error.response.data;
  if (data instanceof Blob) {
    try {
      const text = await data.text();
      data = text ? JSON.parse(text) : null;
    } catch {
      data = null;
    }
  }
  const message = (data as { message?: unknown } | null)?.message;
  return typeof message === 'string' && message.trim() ? message : '操作失败，请检查后重试。';
}

http.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem(TOKEN_KEY);
      localStorage.removeItem(USER_KEY);
    }
    const message = await readErrorMessage(error);
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
  saveUser({ ...response.data.data.user, authMethod: response.data.data.authMethod });
  return response.data.data;
}

export async function loginWithAdminPassword(phone: string, password: string, captcha?: { captchaId?: string; captchaAnswer?: string }) {
  const response = await http.post<ApiResponse<AuthResult>>('/auth/admin-password-login', { phone, password, ...captcha });
  saveToken(response.data.data.token);
  saveUser({ ...response.data.data.user, authMethod: response.data.data.authMethod });
  return response.data.data;
}

export async function getCaptcha() {
  const response = await http.get<ApiResponse<CaptchaChallenge>>('/auth/captcha');
  return response.data.data;
}

export async function changePassword(oldPassword: string, newPassword: string) {
  const response = await http.post<ApiResponse<{ changed: boolean }>>('/auth/change-password', { oldPassword, newPassword });
  return response.data.data;
}

export async function logout() {
  const response = await http.post<ApiResponse<{ loggedOut: boolean }>>('/auth/logout', {});
  return response.data.data;
}

export async function resetTeacherPassword(id: string) {
  return postData<PasswordResetResult>(`/teachers/${id}/reset-password`, {});
}

export async function resetAdminStaffPassword(id: string) {
  return postData<PasswordResetResult>(`/admin-staff/${id}/reset-password`, {});
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

export async function deleteData<T>(url: string) {
  const response = await http.delete<ApiResponse<T>>(url);
  return response.data.data;
}
