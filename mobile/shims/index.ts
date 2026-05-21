import { installWebApiShim } from './webapi';
import { installStubs } from './stubs';

export function installAllShims(): void {
    // stubs first (AppApi/SQLite etc.), then WebApi shim (fetch-based)
    installStubs();
    installWebApiShim();
}
