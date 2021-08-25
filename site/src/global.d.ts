/// <reference types="svelte" />
declare module "*.svg"
declare module '*.html' {
  const value: any;
  export default value;
}

interface ServerConfiguration {
    isProduction: boolean;
    host: string;
    port: string;
}

declare global {
    var SERVER_CONFIG: ServerConfiguration
}
// export {}
