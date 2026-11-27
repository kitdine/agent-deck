import { catalogs } from "./i18n.js";

// Stable row identity: the same model can appear once per client in the
// all-clients view, so the model name alone is not unique.
export function modelKey(model) {
  return `${model.client ?? ""}\u0000${model.model}`;
}

// The all-clients view names each row's client so same-named models stay
// distinguishable; a single-client view keeps the plain model name. The model
// name truncates first and the client suffix is kept visible.
export function ModelLabel({ model, scopeClient, lang }) {
  const raw = model.client?.trim();
  const client = scopeClient === "all" && raw && raw !== "all" ? catalogs[lang].clients[raw] ?? raw : null;
  return (
    <>
      <span className="model-name">{model.model}</span>
      {client && <span className="model-client">· {client}</span>}
    </>
  );
}
