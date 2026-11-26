import { registerHooks } from 'node:module';

export async function loadChannelSchema() {
  const schemaURL = new URL('./schema.ts', import.meta.url).href;
  const hooks = registerHooks({
    resolve(specifier, context, nextResolve) {
      if (context.parentURL === schemaURL) {
        if (specifier === '@/gql/pagination') {
          return { url: new URL('../../../gql/pagination.ts', import.meta.url).href, shortCircuit: true };
        }
        if (specifier === './vertex-service-account') {
          return { url: new URL('./vertex-service-account.ts', import.meta.url).href, shortCircuit: true };
        }
      }
      return nextResolve(specifier, context);
    },
  });

  try {
    return await import(schemaURL);
  } finally {
    hooks.deregister();
  }
}
