/**
 * components/beacon-card.js —— BeaconCard 航标状态卡片
 * 被【航标总览】与【航标详情】共用：展示状态徽标（灭灯/低电/漂移）、
 * 电压与灯状态，点击进入详情页。
 */
(function (global) {
  'use strict';

  const TYPE_LABEL = {
    lighthouse: '灯塔',
    buoy: '浮标',
    daybeacon: '导标',
  };
  const STATUS_LABEL = { active: '在线', offline: '离线' };
  const LAMP_LABEL = { on: '亮', off: '灭' };

  function el(tag, cls, text) {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined && text !== null) node.textContent = text;
    return node;
  }

  /**
   * @param {object} b 航标摘要/详情（含 id/name/type/status/low_power/drifting/lamp_out）
   * @returns {HTMLElement}
   */
  function BeaconCard(b) {
    const card = el('div', 'beacon-card');
    card.dataset.beaconId = b.id;

    // 顶部：名称 + 类型
    const top = el('div', 'top');
    const nameBox = el('div');
    nameBox.appendChild(el('div', 'name', b.name));
    const typeTag = el('span', 'type-tag', TYPE_LABEL[b.type] || b.type);
    nameBox.appendChild(typeTag);
    top.appendChild(nameBox);
    top.appendChild(el('span', 'badge ' + (b.status === 'active' ? 'ok' : 'dim'), STATUS_LABEL[b.status] || b.status));
    card.appendChild(top);

    // 标志徽标
    const flags = el('div', 'flags');
    if (b.lamp_out) flags.appendChild(el('span', 'badge danger', '🔴 灭灯'));
    if (b.low_power) flags.appendChild(el('span', 'badge warn', '🔋 低电'));
    if (b.drifting) flags.appendChild(el('span', 'badge warn', '🧭 漂移'));
    if (!b.lamp_out && !b.low_power && !b.drifting) {
      flags.appendChild(el('span', 'badge ok', '✅ 正常'));
    }
    card.appendChild(flags);

    // 元信息
    const meta = el('div', 'meta');
    if (b.anchor) {
      meta.appendChild(el('span', 'mono', '锚位 ' + b.anchor.lat.toFixed(4) + ', ' + b.anchor.lng.toFixed(4)));
    }
    if (b.lamp_pattern) {
      meta.appendChild(el('span', '', '设定灯质 ' + lampPatternText(b.lamp_pattern)));
    }
    if (b.last_telemetry_at) {
      meta.appendChild(el('span', '', '最近遥测 ' + fmtTime(b.last_telemetry_at)));
    }
    card.appendChild(meta);

    // 底部：电压 + 灯状态 + 详情链接
    const foot = el('div', 'foot');
    const voltBox = el('div');
    const volt = el('span', 'voltage', (b.voltage ? b.voltage.toFixed(2) : '--') + 'V');
    voltBox.appendChild(volt);
    voltBox.appendChild(el('div', 'lamp-state', '灯状态: ' + (LAMP_LABEL[b.lamp_state] || '--')));
    foot.appendChild(voltBox);
    const link = el('a', 'btn secondary sm', '查看详情 →');
    link.href = '/beacons/' + encodeURIComponent(b.id);
    foot.appendChild(link);
    card.appendChild(foot);

    return card;
  }

  function lampPatternText(p) {
    if (!p) return '--';
    return '闪' + fmt1(p.flash_sec) + 's/灭' + fmt1(p.eclipse_sec) + 's';
  }

  function fmt1(v) { return (Math.round(v * 10) / 10).toFixed(1); }

  function fmtTime(iso) {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    const pad = (n) => String(n).padStart(2, '0');
    return pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes());
  }

  global.BeaconCard = BeaconCard;
  global.BeaconCardUtils = { fmtTime, lampPatternText };
})(window);
