const MODELS_QUERY = `
  query LocalModels($after: Cursor) {
    models(first: 100, after: $after) {
      edges {
        node { modelID status catalogAliases catalogPrices { field value follow provider modelID canonical } }
      }
      pageInfo { hasNextPage endCursor }
    }
  }
`;

const CATALOG_QUERY = `
  query CatalogOffers($provider: String!, $after: String, $refresh: Boolean!) {
    modelCatalogOffers(providers: [$provider], first: 100, after: $after, refresh: $refresh) {
      source fetchedAt nextCursor
      offers { modelID canonical rates { field base } }
    }
  }
`;

function query(ctx, document, variables) {
  const { data, errors } = ctx.graphql(document, variables);
  if (errors?.length) throw new Error(errors[0].message);
  return data;
}

function listLocalModels(ctx) {
  const models = [];
  let after = null;
  do {
    const { models: page } = query(ctx, MODELS_QUERY, { after });
    for (const { node } of page.edges) models.push(node);
    after = page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null;
  } while (after);
  return models;
}

function listUpstreamOffers(ctx, provider) {
  const offers = new Map();
  let after = null;
  let fetchedAt;
  do {
    const { modelCatalogOffers: page } = query(ctx, CATALOG_QUERY, { provider, after, refresh: after === null });
    if (page.source !== "upstream") throw new Error("The upstream model catalog is unavailable");
    for (const offer of page.offers) offers.set(offer.modelID, offer);
    fetchedAt = page.fetchedAt;
    after = page.nextCursor;
  } while (after);
  return { offers, fetchedAt };
}

export function onTask(ctx) {
  const provider = (ctx.config.provider ?? "openai").trim();
  if (!provider) throw new Error("Choose a catalog provider ID");

  const models = listLocalModels(ctx);
  const { offers, fetchedAt } = listUpstreamOffers(ctx, provider);
  const importedIDs = new Set();
  let changed = 0;
  for (const model of models) {
    importedIDs.add(model.modelID);
    for (const alias of model.catalogAliases) importedIDs.add(alias);
    if (model.status === "archived") continue;
    const priceChanged = model.catalogPrices.some((price) => {
      if (!price.follow || price.provider !== provider) return false;
      const offer = offers.get(price.modelID);
      if (!offer || offer.canonical !== price.canonical) return false;
      const rate = offer.rates.find((rate) => rate.field === price.field);
      return rate?.base != null && rate.base !== price.value;
    });
    if (priceChanged) changed++;
  }

  let available = 0;
  for (const id of offers.keys()) {
    if (!importedIDs.has(id)) available++;
  }

  ctx.kv.set("summary", JSON.stringify({ provider, fetchedAt, available, changed }));
  const count = available + changed;
  ctx.ui.setBadge({ type: "action", key: "catalog-check" }, count ? { count } : null);
}
