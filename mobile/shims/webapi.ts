// Replaces window.WebApi (C# HttpClient) with browser fetch → /api/v1/proxy/*

const VRC_API_PREFIX = 'https://api.vrchat.cloud/api/1/';
const PROXY_BASE = '/api/v1/proxy/';

interface ExecuteOptions {
    url: string;
    method?: string;
    headers?: Record<string, string>;
    body?: string;
}

interface ExecuteResult {
    status: number;
    data?: string;
}

class WebApiShim {
    clearCookies(): void {}
    getCookies(): string { return ''; }
    setCookies(_cookie: string): void {}

    async execute(options: ExecuteOptions): Promise<ExecuteResult> {
        const { url, method = 'GET', headers = {}, body } = options;

        // Strip the VRChat API base to get the relative path + query string
        const relPath = url.startsWith(VRC_API_PREFIX)
            ? url.slice(VRC_API_PREFIX.length)
            : url;

        const proxyUrl = PROXY_BASE + relPath;

        const response = await fetch(proxyUrl, {
            method,
            headers: {
                'Content-Type': 'application/json',
                ...headers
            },
            body: method !== 'GET' ? body : undefined,
            credentials: 'include'
        });

        const data = await response.text();
        return { status: response.status, data };
    }
}

export function installWebApiShim(): void {
    const shim = new WebApiShim();
    // The shared src/services/webapi.js references `WebApi` directly (unqualified)
    // so we expose on window for the legacy code
    (window as any).WebApi = shim;
    (window as any).webApiShim = shim;
}
