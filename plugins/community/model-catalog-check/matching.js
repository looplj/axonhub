function lastSegment(id) {
  return id.slice(id.lastIndexOf("/") + 1) || id;
}

function append(map, key, value) {
  if (!map.has(key)) map.set(key, []);
  map.get(key).push(value);
}

// Match the native importer's exact-ID precedence and canonical alias rules.
function compatible(ids, exact, canonical) {
  const known = ["", "", ""];
  for (const id of ids) {
    let offers = exact.get(id);
    if (!offers) {
      offers = canonical.get(id);
      if (!offers || !offers.every((offer) => offer.id === offers[0].id && lastSegment(offer.id) === lastSegment(id))) return false;
    }
    for (const offer of offers) {
      if (!offer.canonical) return false;
      const identity = JSON.parse(offer.identity);
      for (let i = 0; i < known.length; i++) {
        if (!identity[i]) continue;
        if (known[i] && known[i] !== identity[i]) return false;
        known[i] = identity[i];
      }
    }
  }
  return true;
}

function groupCandidates(candidates, imported, channels, ignorePrefix) {
  const groups = new Map();
  for (const id of candidates) {
    if (!imported.has(id)) append(groups, ignorePrefix && channels.has(id) ? lastSegment(id) : id, id);
  }
  return groups;
}

export function possibleMergeIDs(channels, imported, hasProviders) {
  const ids = new Set();
  for (const [target, members] of groupCandidates(channels, imported, channels, true)) {
    // A subscribed provider can still contribute the bare target for a prefixed channel ID.
    const canMerge = members.length > 1 || (hasProviders && members[0] !== target);
    if (!canMerge || imported.has(target)) continue;
    ids.add(target);
    for (const member of members) ids.add(member);
  }
  return ids;
}

export function countAvailable(candidates, imported, channels, ignorePrefix, offers) {
  const groups = groupCandidates(candidates, imported, channels, ignorePrefix);
  if (!ignorePrefix) return groups.size;
  const exact = new Map();
  const canonical = new Map();
  for (const offer of offers) {
    append(exact, offer.id, offer);
    if (offer.canonical) append(canonical, offer.canonical, offer);
  }
  let count = 0;
  for (const [target, ids] of groups) {
    count += ids.length > 1 && !imported.has(target) && compatible(ids, exact, canonical) ? 1 : ids.length;
  }
  return count;
}
