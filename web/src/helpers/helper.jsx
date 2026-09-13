import {API} from './api';
import {CHANNEL_PROTOCOL_OPTIONS} from '../constants';

const CHANNEL_PROTOCOLS_STORAGE_KEY = 'channel_protocol_options';
const CHANNEL_PROTOCOL_DISPLAY_MAP = {
  ali: {name: 'ali', text: 'QianWen'},
  zhipu: {name: 'zhipu', text: 'ZhiPu'},
  tencent: {name: 'tencent', text: 'Hunyuan'},
  volcengine: {name: 'volcengine', text: 'VolcEngine'},
  doubao: {name: 'volcengine', text: 'VolcEngine'},
};

let channelProtocolOptions = undefined;
let channelProtocolMapByName = undefined;

const normalizeProtocolName = (value) => {
  if (value === undefined || value === null) {
    return '';
  }
  const raw = String(value).trim().toLowerCase();
  if (raw === '') {
    return '';
  }
  return raw;
};

const normalizeChannelOption = (item) => {
  if (!item || typeof item !== 'object') {
    return null;
  }
  const protocol = normalizeProtocolName(item.value ?? item.key ?? item.name);
  if (protocol === '') {
    return null;
  }
  const displayConfig = CHANNEL_PROTOCOL_DISPLAY_MAP[protocol] || null;
  const visibleProtocol = displayConfig?.name || protocol;
  const text = String(
    displayConfig?.text ?? item.text ?? item.label ?? item.name ?? protocol
  ).trim();
  return {
    key: visibleProtocol,
    value: visibleProtocol,
    text: text || protocol,
    color: typeof item.color === 'string' ? item.color.trim() : '',
    description:
      typeof item.description === 'string' ? item.description.trim() : '',
    tip: typeof item.tip === 'string' ? item.tip.trim() : '',
    name: visibleProtocol,
    sort_order: Number.isFinite(Number(item.sort_order))
      ? Number(item.sort_order)
      : Number.MAX_SAFE_INTEGER,
  };
};

const normalizeChannelOptions = (items) => {
  if (!Array.isArray(items)) {
    return [];
  }
  const unique = new Map();
  items.forEach((item) => {
    const normalized = normalizeChannelOption(item);
    if (!normalized) {
      return;
    }
    if (!unique.has(normalized.name)) {
      unique.set(normalized.name, normalized);
    }
  });
  return Array.from(unique.values()).sort((a, b) => {
    if (a.sort_order !== b.sort_order) {
      return a.sort_order - b.sort_order;
    }
    return a.text.localeCompare(b.text);
  });
};

const updateChannelOptionCache = (items, persist = true) => {
  const normalizedItems = normalizeChannelOptions(items);
  const normalized =
    normalizedItems.length > 0
      ? normalizedItems
      : normalizeChannelOptions(CHANNEL_PROTOCOL_OPTIONS);
  channelProtocolOptions = normalized;
  channelProtocolMapByName = {};
  channelProtocolOptions.forEach((option) => {
    channelProtocolMapByName[option.name] = option;
  });
  if (persist && typeof window !== 'undefined') {
    localStorage.setItem(
      CHANNEL_PROTOCOLS_STORAGE_KEY,
      JSON.stringify(channelProtocolOptions)
    );
  }
  return channelProtocolOptions;
};

const ensureChannelOptionsLoaded = () => {
  if (channelProtocolOptions !== undefined) {
    return channelProtocolOptions;
  }
  if (typeof window !== 'undefined') {
    const stored = localStorage.getItem(CHANNEL_PROTOCOLS_STORAGE_KEY);
    if (stored) {
      try {
        const parsed = JSON.parse(stored);
        if (Array.isArray(parsed) && parsed.length > 0) {
          return updateChannelOptionCache(parsed, false);
        }
      } catch (e) {
        // ignore invalid cached payload
      }
    }
  }
  return updateChannelOptionCache(CHANNEL_PROTOCOL_OPTIONS, false);
};

export function getChannelProtocolOptions() {
  return ensureChannelOptionsLoaded();
}

export function getChannelProtocolOption(protocol) {
  ensureChannelOptionsLoaded();
  const normalized = normalizeProtocolName(protocol);
  if (normalized !== '' && channelProtocolMapByName[normalized]) {
    return channelProtocolMapByName[normalized];
  }
  return undefined;
}

export async function loadChannelProtocolOptions() {
  try {
    const res = await API.get('/api/v1/admin/channel/protocols');
    const {success, data} = res.data || {};
    if (success && Array.isArray(data) && data.length > 0) {
      return updateChannelOptionCache(data, true);
    }
  } catch (e) {
    // keep fallback cache
  }
  return ensureChannelOptionsLoaded();
}
