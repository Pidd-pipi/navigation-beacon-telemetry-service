/**
 * app.js —— 前端路由与页面渲染
 * 纯原生 JS 实现路径路由：/、/beacons/{id}、/abnormalities、/tasks、/commands。
 * 各页面真实消费后端 API，并复用共享组件（BeaconCard/TelemetryChart/CommandPanel）
 * 与共享 Hooks（useBeacons/useTasks）。
 */
(function () {
  'use strict';

  const root = document.getElementById('app');
  let cleanups = [];

  function el(tag, cls, text) {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined && text !== null) node.textContent = text;
    return node;
  }

  function escapeHtml(s) {
    return String(s == null ? '' : s)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }

  function fmtTime(iso) {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso || '--';
    const pad = (n) => String(n).padStart(2, '0');
    return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) +
      ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
  }

  function registerCleanup(fn) { cleanups.push(fn); }

  function cleanup() {
    cleanups.forEach((fn) => { try { fn(); } catch (e) { /* ignore */ } });
    cleanups = [];
  }

  function navigate(path) {
    history.pushState(null, '', path);
    render();
  }

  function navActive() {
    const path = location.pathname;
    document.querySelectorAll('#app-nav a').forEach((a) => {
      const route = a.dataset.route;
      const active = route === '/' ? path === '/' : path.startsWith(route);
      a.classList.toggle('active', active);
    });
  }

  async function render() {
    cleanup();
    navActive();
    const path = location.pathname;
    root.innerHTML = '<div class="loading">加载中…</div>';
    try {
      if (path === '/') {
        await renderOverview();
      } else if (/^\/beacons\/[^/]+$/.test(path)) {
        await renderBeaconDetail(decodeURIComponent(path.split('/')[2]));
      } else if (path === '/abnormalities') {
        await renderAbnormalities();
      } else if (path === '/tasks') {
        await renderTasks();
      } else if (path === '/commands') {
        await renderCommands();
      } else {
        root.innerHTML = '<div class="empty">页面不存在: ' + escapeHtml(path) + '</div>';
      }
    } catch (e) {
      root.innerHTML = '<div class="msg err">页面渲染失败: ' + escapeHtml(e.message) + '</div>';
    }
  }

  /* ============================ 总览页 ============================ */
  async function renderOverview() {
    const wrap = el('div');
    const overview = await API.overview();
    const b = overview.beacons;

    const stats = el('div', 'stat-grid');
    stats.appendChild(statTile(b.total, '航标总数', ''));
    stats.appendChild(statTile(b.lamp_out, '灭灯', 'danger'));
    stats.appendChild(statTile(b.low_power, '低电', 'warn'));
    stats.appendChild(statTile(b.drifting, '漂移', 'warn'));
    stats.appendChild(statTile(overview.abnormalities.open, '未解决异常', overview.abnormalities.open ? 'warn' : 'ok'));
    stats.appendChild(statTile(overview.tasks.open, '进行中任务', overview.tasks.open ? '' : 'ok'));
    stats.appendChild(statTile(overview.tasks.overdue, '超期任务', overview.tasks.overdue ? 'danger' : 'ok'));
    stats.appendChild(statTile(overview.commands.pending, '待回执指令', overview.commands.pending ? 'warn' : 'ok'));
    wrap.appendChild(stats);

    // 航标卡片（useBeacons 轮询共享）
    const beaconPanel = el('div', 'panel');
    beaconPanel.appendChild(panelHead('🚩 航标状态'));
    const beaconBody = el('div', 'panel-body');
    beaconPanel.appendChild(beaconBody);
    const grid = el('div', 'card-grid');
    beaconBody.appendChild(grid);
    wrap.appendChild(beaconPanel);

    const renderGrid = (state) => {
      grid.innerHTML = '';
      if (state.error) {
        grid.appendChild(el('div', 'msg err', '航标加载失败: ' + state.error));
        return;
      }
      if (!state.beacons.length) {
        grid.appendChild(el('div', 'empty', '暂无航标'));
        return;
      }
      const lampOutIds = new Set(
        (overview.abnormalities.by_type && overview.abnormalities.by_type.lamp_out
          ? Object.keys(overview.beacons.summaries).filter((k) => overview.beacons.summaries[k].lamp_out)
          : [])
      );
      state.beacons.forEach((beacon) => {
        const sum = (overview.beacons.summaries || []).find((s) => s.id === beacon.id) || {};
        const card = BeaconCard({
          ...beacon,
          status: sum.status || beacon.status,
          lamp_out: sum.lamp_out || lampOutIds.has(beacon.id) || false,
          low_power: sum.low_power || beacon.low_power || false,
          drifting: sum.drifting || beacon.drifting || false,
          voltage: sum.voltage,
          lamp_state: sum.lamp_state,
        });
        grid.appendChild(card);
      });
    };
    const hook = useBeacons(8000, renderGrid);
    hook.start();
    registerCleanup(hook.stop);

    // 进行中任务摘要（useTasks 共享）
    const taskPanel = el('div', 'panel');
    taskPanel.appendChild(panelHead('📋 进行中处置任务'));
    const taskBody = el('div', 'panel-body');
    taskPanel.appendChild(taskBody);
    wrap.appendChild(taskPanel);
    const taskHook = useTasks({}, 12000, (state) => {
      taskBody.innerHTML = '';
      taskBody.appendChild(taskTable(state.tasks.filter((t) => t.status !== 'closed'), { showActions: false }));
    });
    taskHook.start();
    registerCleanup(taskHook.stop);

    // 最近遥控指令
    const cmdPanel = el('div', 'panel');
    cmdPanel.appendChild(panelHead('🛰️ 最近遥控指令'));
    const cmdBody = el('div', 'panel-body');
    cmdPanel.appendChild(cmdBody);
    wrap.appendChild(cmdPanel);
    cmdBody.appendChild(commandTable(overview.recent_commands || [], { showAck: false }));

    root.innerHTML = '';
    root.appendChild(wrap);
  }

  function statTile(num, label, cls) {
    const tile = el('div', 'stat-tile');
    tile.appendChild(el('div', 'num ' + cls, String(num)));
    tile.appendChild(el('div', 'label', label));
    return tile;
  }

  function panelHead(title) {
    const head = el('div', 'panel-head');
    head.appendChild(el('h2', '', title));
    return head;
  }

  /* ============================ 详情页 ============================ */
  async function renderBeaconDetail(id) {
    const [detail, telemetry, beacons] = await Promise.all([
      API.beacon(id),
      API.beaconTelemetry(id, 120),
      API.beacons(),
    ]);
    const beacon = detail.beacon;
    const last = detail.last_telemetry;
    const summary = {
      ...beacon,
      status: detail.effective_status,
      lamp_out: (detail.open_abnormalities || []).some((a) => a.type === 'lamp_out'),
      voltage: last ? last.voltage : undefined,
      lamp_state: last ? last.lamp_state : undefined,
    };

    const wrap = el('div');
    const grid = el('div', 'detail-grid');

    // 左列：卡片 + 台账信息 + 快捷导航
    const left = el('div');
    left.appendChild(BeaconCard(summary));

    const info = el('div', 'panel');
    info.appendChild(panelHead('📋 台账信息'));
    const infoBody = el('div', 'panel-body');
    const kv = el('dl', 'kv');
    kv.appendChild(el('dt', '', '航标 ID'));
    kv.appendChild(el('dd', 'mono', beacon.id));
    kv.appendChild(el('dt', '', '锚位'));
    kv.appendChild(el('dd', 'mono', beacon.anchor.lat.toFixed(6) + ', ' + beacon.anchor.lng.toFixed(6)));
    kv.appendChild(el('dt', '', '漂移半径'));
    kv.appendChild(el('dd', '', beacon.drift_radius_m + ' 米'));
    kv.appendChild(el('dt', '', '设定灯质'));
    kv.appendChild(el('dd', '', BeaconCardUtils.lampPatternText(beacon.lamp_pattern)));
    kv.appendChild(el('dt', '', '遥测条数'));
    kv.appendChild(el('dd', '', String(detail.telemetry_count)));
    infoBody.appendChild(kv);
    info.appendChild(infoBody);
    left.appendChild(info);

    // 快捷切换航标
    const navPanel = el('div', 'panel');
    navPanel.appendChild(panelHead('🧭 其他航标'));
    const navBody = el('div', 'panel-body');
    (beacons || []).forEach((b) => {
      const link = el('a', '', (b.id === id ? '▶ ' : '') + b.name + ' (' + b.id + ')');
      link.href = '/beacons/' + encodeURIComponent(b.id);
      link.addEventListener('click', (e) => { e.preventDefault(); navigate(link.href); });
      const item = el('div', '');
      item.style.padding = '4px 0';
      item.appendChild(link);
      navBody.appendChild(item);
    });
    navPanel.appendChild(navBody);
    left.appendChild(navPanel);
    grid.appendChild(left);

    // 右列：图表 + 灯质校验 + 遥控面板 + 遥测明细
    const right = el('div');

    const chartPanel = el('div', 'panel');
    chartPanel.appendChild(panelHead('📈 遥测趋势（电压/灯状态）'));
    const chartBody = el('div', 'panel-body');
    chartBody.appendChild(TelemetryChart(telemetry || [], {
      title: '近 ' + (telemetry ? telemetry.length : 0) + ' 条遥测',
      lampPattern: BeaconCardUtils.lampPatternText(beacon.lamp_pattern),
    }));
    chartPanel.appendChild(chartBody);
    right.appendChild(chartPanel);

    // 灯质校验
    const checkPanel = el('div', 'panel');
    checkPanel.appendChild(panelHead('💡 灯质校验'));
    const checkBody = el('div', 'panel-body');
    if (last && last.measured_pattern) {
      const ok = Math.abs(last.measured_pattern.flash_sec - beacon.lamp_pattern.flash_sec) <= 0.5 &&
        Math.abs(last.measured_pattern.eclipse_sec - beacon.lamp_pattern.eclipse_sec) <= 0.5;
      checkBody.appendChild(el('div', 'kv'));
      const kv2 = el('dl', 'kv');
      kv2.appendChild(el('dt', '', '设定灯质'));
      kv2.appendChild(el('dd', '', BeaconCardUtils.lampPatternText(beacon.lamp_pattern)));
      kv2.appendChild(el('dt', '', '实测灯质'));
      kv2.appendChild(el('dd', '', BeaconCardUtils.lampPatternText(last.measured_pattern)));
      kv2.appendChild(el('dt', '', '校验结果'));
      kv2.appendChild(el('dd', '', ok ? '✅ 在容差(±0.5s)内' : '❌ 灯质偏差超容差'));
      checkBody.appendChild(kv2);
    } else {
      checkBody.appendChild(el('div', 'empty', '暂无实测灯质数据'));
    }
    if (last && last.violations && last.violations.length) {
      const ul = el('ul', 'violations');
      ul.appendChild(el('li', '', '最近遥测违规项：'));
      last.violations.forEach((v) => ul.appendChild(el('li', '', v)));
      checkBody.appendChild(ul);
    }
    checkPanel.appendChild(checkBody);
    right.appendChild(checkPanel);

    // 遥控面板（共享组件）
    right.appendChild(CommandPanel({ beaconId: id, lampPattern: beacon.lamp_pattern }));

    // 未解决异常
    const abnPanel = el('div', 'panel');
    abnPanel.appendChild(panelHead('⚠️ 未解决异常'));
    const abnBody = el('div', 'panel-body');
    abnPanel.appendChild(abnBody);
    if (detail.open_abnormalities && detail.open_abnormalities.length) {
      abnBody.appendChild(abnormalityTable(detail.open_abnormalities, { showResolve: false }));
    } else {
      abnBody.appendChild(el('div', 'empty', '无未解决异常'));
    }
    right.appendChild(abnPanel);

    grid.appendChild(right);
    wrap.appendChild(grid);

    // 模拟遥测上报表单（消费 POST /api/beacons/{id}/telemetry）
    wrap.appendChild(telemetrySimulator(id, beacon, telemetry, chartBody));

    root.innerHTML = '';
    root.appendChild(wrap);
  }

  /* ============================ 异常台账 ============================ */
  async function renderAbnormalities() {
    const wrap = el('div');
    const typeFilter = el('select');
    [['', '全部类型'], ['lamp_mismatch', '灯质偏差'], ['lamp_out', '灭灯'], ['low_voltage', '低电压'], ['drift', '漂移']]
      .forEach(([v, label]) => {
        const opt = el('option', '', label);
        opt.value = v;
        typeFilter.appendChild(opt);
      });
    const statusFilter = el('select');
    [['', '全部状态'], ['open', '未解决'], ['resolved', '已解决']].forEach(([v, label]) => {
      const opt = el('option', '', label);
      opt.value = v;
      statusFilter.appendChild(opt);
    });

    const filterBar = el('div', 'form-row');
    const f1 = el('div', 'field'); f1.appendChild(el('label', '', '类型')); f1.appendChild(typeFilter);
    const f2 = el('div', 'field'); f2.appendChild(el('label', '', '状态')); f2.appendChild(statusFilter);
    const refreshBtn = el('button', 'btn secondary', '刷新');
    filterBar.appendChild(f1); filterBar.appendChild(f2); filterBar.appendChild(refreshBtn);
    wrap.appendChild(filterBar);

    const listPanel = el('div', 'panel');
    listPanel.appendChild(panelHead('📒 灯质异常台账'));
    const listBody = el('div', 'panel-body');
    listPanel.appendChild(listBody);
    wrap.appendChild(listPanel);

    async function load() {
      listBody.innerHTML = '<div class="loading">加载中…</div>';
      try {
        const items = await API.abnormalities({ type: typeFilter.value, status: statusFilter.value });
        listBody.innerHTML = '';
        if (!items.length) {
          listBody.appendChild(el('div', 'empty', '暂无异常记录'));
        } else {
          listBody.appendChild(abnormalityTable(items, { showResolve: true }));
        }
      } catch (e) {
        listBody.innerHTML = '';
        listBody.appendChild(el('div', 'msg err', '加载失败: ' + e.message));
      }
    }
    typeFilter.addEventListener('change', load);
    statusFilter.addEventListener('change', load);
    refreshBtn.addEventListener('click', load);
    await load();

    // TelemetryChart 共享：按航标切换查看趋势
    const chartPanel = el('div', 'panel');
    chartPanel.appendChild(panelHead('📈 航标遥测趋势（TelemetryChart 共享组件）'));
    const chartBody = el('div', 'panel-body');
    chartPanel.appendChild(chartBody);
    wrap.appendChild(chartPanel);

    const beacons = await API.beacons();
    const beaconSelect = el('select');
    const noneOpt = el('option', '', '选择航标查看趋势');
    noneOpt.value = '';
    beaconSelect.appendChild(noneOpt);
    beacons.forEach((b) => {
      const opt = el('option', '', b.name + ' (' + b.id + ')');
      opt.value = b.id;
      beaconSelect.appendChild(opt);
    });
    const chartRow = el('div', 'form-row');
    const cf = el('div', 'field');
    cf.appendChild(el('label', '', '航标'));
    cf.appendChild(beaconSelect);
    chartRow.appendChild(cf);
    chartBody.appendChild(chartRow);
    const chartSlot = el('div');
    chartBody.appendChild(chartSlot);
    beaconSelect.addEventListener('change', async () => {
      chartSlot.innerHTML = '';
      if (!beaconSelect.value) return;
      const tel = await API.beaconTelemetry(beaconSelect.value, 80);
      const b = beacons.find((x) => x.id === beaconSelect.value);
      chartSlot.appendChild(TelemetryChart(tel || [], {
        title: b.name,
        lampPattern: BeaconCardUtils.lampPatternText(b.lamp_pattern),
      }));
    });

    // 手工登记异常
    const createPanel = el('div', 'panel');
    createPanel.appendChild(panelHead('➕ 手工登记异常'));
    const createBody = el('div', 'panel-body');
    const cbSel = el('select');
    beacons.forEach((b) => {
      const opt = el('option', '', b.name + ' (' + b.id + ')');
      opt.value = b.id;
      cbSel.appendChild(opt);
    });
    const ctSel = el('select');
    [['lamp_mismatch', '灯质偏差'], ['lamp_out', '灭灯'], ['low_voltage', '低电压'], ['drift', '漂移']]
      .forEach(([v, label]) => {
        const opt = el('option', '', label);
        opt.value = v;
        ctSel.appendChild(opt);
      });
    const detailInput = el('input');
    detailInput.placeholder = '异常描述（可选）';
    const createRow = el('div', 'form-row');
    const c1 = el('div', 'field'); c1.appendChild(el('label', '', '航标')); c1.appendChild(cbSel);
    const c2 = el('div', 'field'); c2.appendChild(el('label', '', '类型')); c2.appendChild(ctSel);
    const c3 = el('div', 'field'); c3.appendChild(el('label', '', '描述')); c3.appendChild(detailInput);
    const createBtn = el('button', 'btn', '登记异常');
    createRow.appendChild(c1); createRow.appendChild(c2); createRow.appendChild(c3); createRow.appendChild(createBtn);
    createBody.appendChild(createRow);
    const createMsg = el('div');
    createBody.appendChild(createMsg);
    createPanel.appendChild(createBody);
    wrap.appendChild(createPanel);

    createBtn.addEventListener('click', async () => {
      createMsg.className = '';
      try {
        const ab = await API.createAbnormality({
          beacon_id: cbSel.value,
          type: ctSel.value,
          detail: detailInput.value || '手工登记',
        });
        createMsg.className = 'msg ok';
        createMsg.textContent = '✅ 已登记异常 ' + ab.id + (ab.type === 'lamp_out' ? '（已自动生成处置任务）' : '');
        detailInput.value = '';
        await load();
      } catch (e) {
        createMsg.className = 'msg err';
        createMsg.textContent = '❌ 登记失败: ' + e.message;
      }
    });

    root.innerHTML = '';
    root.appendChild(wrap);
  }

  /* ============================ 处置任务 ============================ */
  async function renderTasks() {
    const wrap = el('div');
    const statusFilter = el('select');
    [['', '全部状态'], ['created', '已生成'], ['assigned', '已派发'], ['repaired', '已修复'], ['verified', '已复测'], ['closed', '已关闭']]
      .forEach(([v, label]) => {
        const opt = el('option', '', label);
        opt.value = v;
        statusFilter.appendChild(opt);
      });
    const filterBar = el('div', 'form-row');
    const f1 = el('div', 'field'); f1.appendChild(el('label', '', '状态')); f1.appendChild(statusFilter);
    const refreshBtn = el('button', 'btn secondary', '刷新');
    filterBar.appendChild(f1); filterBar.appendChild(refreshBtn);
    wrap.appendChild(filterBar);

    const panel = el('div', 'panel');
    panel.appendChild(panelHead('📋 处置任务（状态机流转）'));
    const body = el('div', 'panel-body');
    panel.appendChild(body);
    wrap.appendChild(panel);

    const hook = useTasks({}, 8000, (state) => {
      body.innerHTML = '';
      if (state.error) {
        body.appendChild(el('div', 'msg err', state.error));
        return;
      }
      const items = state.tasks.filter((t) => !statusFilter.value || t.status === statusFilter.value);
      body.appendChild(taskTable(items, { showActions: true }));
    });
    statusFilter.addEventListener('change', () => hook.setFilter({ status: statusFilter.value }));
    hook.start();
    registerCleanup(hook.stop);

    // 生成任务
    const createPanel = el('div', 'panel');
    createPanel.appendChild(panelHead('➕ 为异常生成处置任务'));
    const createBody = el('div', 'panel-body');
    const abns = await API.abnormalities({ status: 'open' });
    const abSel = el('select');
    if (!abns.length) {
      abSel.appendChild(el('option', '', '暂无未解决异常'));
    }
    abns.forEach((a) => {
      const opt = el('option', '', a.id + ' · ' + a.type + ' · 航标 ' + a.beacon_id);
      opt.value = a.id;
      abSel.appendChild(opt);
    });
    const row = el('div', 'form-row');
    const f = el('div', 'field'); f.appendChild(el('label', '', '异常')); f.appendChild(abSel);
    const btn = el('button', 'btn', '生成任务');
    row.appendChild(f); row.appendChild(btn);
    createBody.appendChild(row);
    const msg = el('div');
    createBody.appendChild(msg);
    createPanel.appendChild(createBody);
    wrap.appendChild(createPanel);
    btn.addEventListener('click', async () => {
      msg.className = '';
      if (!abSel.value) { msg.className = 'msg info'; msg.textContent = '请先选择异常'; return; }
      try {
        const task = await API.createTask(abSel.value);
        msg.className = 'msg ok';
        msg.textContent = '✅ 已生成任务 ' + task.id + '，状态: ' + task.status + '，级别: ' + task.level;
        hook.refresh();
      } catch (e) {
        msg.className = 'msg err';
        msg.textContent = '❌ 生成失败: ' + e.message;
      }
    });

    root.innerHTML = '';
    root.appendChild(wrap);
  }

  /* ============================ 遥控记录 ============================ */
  async function renderCommands() {
    const wrap = el('div');
    const beacons = await API.beacons();

    // 共享 CommandPanel（选择航标下发）
    wrap.appendChild(CommandPanel({ beacons }));

    const panel = el('div', 'panel');
    panel.appendChild(panelHead('🛰️ 遥控指令记录'));
    const body = el('div', 'panel-body');
    panel.appendChild(body);
    wrap.appendChild(panel);

    async function load() {
      body.innerHTML = '<div class="loading">加载中…</div>';
      try {
        const items = await API.commands({});
        body.innerHTML = '';
        if (!items.length) {
          body.appendChild(el('div', 'empty', '暂无遥控记录'));
        } else {
          body.appendChild(commandTable(items, { showAck: true }));
        }
      } catch (e) {
        body.innerHTML = '';
        body.appendChild(el('div', 'msg err', '加载失败: ' + e.message));
      }
    }
    await load();

    const audits = await API.audits(10);
    const auditPanel = el('div', 'panel');
    auditPanel.appendChild(panelHead('📜 最近审计日志'));
    const auditBody = el('div', 'panel-body');
    auditPanel.appendChild(auditBody);
    auditBody.appendChild(auditTable(audits || []));
    wrap.appendChild(auditPanel);

    root.innerHTML = '';
    root.appendChild(wrap);
  }

  /* ============================ 表格渲染 ============================ */
  function abnormalityTable(items, opts) {
    const wrap = el('div', 'table-wrap');
    if (!items.length) { wrap.appendChild(el('div', 'empty', '暂无记录')); return wrap; }
    const table = el('table', 'data');
    table.innerHTML =
      '<thead><tr><th>ID</th><th>航标</th><th>类型</th><th>状态</th><th>首次发现</th><th>最后发现</th><th>操作</th></tr></thead>';
    const tbody = el('tbody');
    items.forEach((a) => {
      const tr = el('tr');
      tr.appendChild(el('td', 'mono', a.id));
      tr.appendChild(el('td', '', a.beacon_id));
      tr.appendChild(el('td', '', a.type));
      tr.appendChild(el('td', '', a.status));
      tr.appendChild(el('td', 'mono', fmtTime(a.first_seen_at)));
      tr.appendChild(el('td', 'mono', fmtTime(a.last_seen_at)));
      const tdOp = el('td');
      if (opts.showResolve && a.status === 'open') {
        const btn = el('button', 'btn sm', '解决');
        btn.addEventListener('click', async () => {
          try {
            await API.resolveAbnormality(a.id, '人工确认解决');
            render();
          } catch (e) {
            alert('解决失败: ' + e.message);
          }
        });
        tdOp.appendChild(btn);
      }
      tr.appendChild(tdOp);
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
    wrap.appendChild(table);
    return wrap;
  }

  const TASK_STATUS = { created: '已生成', assigned: '已派发', repaired: '已修复', verified: '已复测', closed: '已关闭' };
  const TASK_LEVEL = { normal: '普通', urgent: '紧急' };

  function taskTable(items, opts) {
    const wrap = el('div', 'table-wrap');
    if (!items.length) { wrap.appendChild(el('div', 'empty', '暂无任务')); return wrap; }
    const table = el('table', 'data');
    table.innerHTML =
      '<thead><tr><th>ID</th><th>标题</th><th>级别</th><th>状态</th><th>派发人</th><th>期限</th><th>复测结果</th><th>操作</th></tr></thead>';
    const tbody = el('tbody');
    items.forEach((t) => {
      const tr = el('tr');
      tr.appendChild(el('td', 'mono', t.id));
      tr.appendChild(el('td', '', t.title));
      tr.appendChild(el('td', '', TASK_LEVEL[t.level] || t.level));
      tr.appendChild(el('td', '', TASK_STATUS[t.status] || t.status));
      tr.appendChild(el('td', '', t.assignee || '--'));
      tr.appendChild(el('td', 'mono', fmtTime(t.deadline)));
      tr.appendChild(el('td', '', t.verify_result || '--'));
      const tdOp = el('td');
      if (opts.showActions) {
        if (t.status === 'created') {
          tdOp.appendChild(actionBtn('派发', () => API.assignTask(t.id, '值班员'), 'secondary'));
        } else if (t.status === 'assigned') {
          tdOp.appendChild(actionBtn('修复', () => API.repairTask(t.id, '现场修复完成'), 'secondary'));
        } else if (t.status === 'repaired') {
          tdOp.appendChild(actionBtn('复测关闭', () => API.verifyTask(t.id, '复测灯质正常'), 'secondary'));
        } else if (t.status === 'verified') {
          tdOp.appendChild(actionBtn('关闭', () => API.closeTask(t.id), 'secondary'));
        }
        if (t.status !== 'closed' && !t.escalated) {
          tdOp.appendChild(actionBtn('升级', () => API.escalateTask(t.id), 'warn'));
        }
      }
      tr.appendChild(tdOp);
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
    wrap.appendChild(table);
    return wrap;
  }

  function actionBtn(label, fn, cls) {
    const btn = el('button', 'btn sm ' + (cls || ''), label);
    btn.style.marginRight = '4px';
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      try {
        await fn();
        render();
      } catch (e) {
        alert('操作失败: ' + e.message);
        btn.disabled = false;
      }
    });
    return btn;
  }

  const ACK_STATUS = { pending: '待回执', success: '成功', failed: '失败' };
  const CMD_TYPE = { on: '开灯', off: '关灯', switch_pattern: '切换灯质' };

  function commandTable(items, opts) {
    const wrap = el('div', 'table-wrap');
    if (!items.length) { wrap.appendChild(el('div', 'empty', '暂无记录')); return wrap; }
    const table = el('table', 'data');
    table.innerHTML =
      '<thead><tr><th>ID</th><th>航标</th><th>类型</th><th>状态</th><th>重试</th><th>下发时间</th><th>回执</th><th>操作</th></tr></thead>';
    const tbody = el('tbody');
    items.forEach((c) => {
      const tr = el('tr');
      tr.appendChild(el('td', 'mono', c.id));
      tr.appendChild(el('td', 'mono', c.beacon_id));
      tr.appendChild(el('td', '', CMD_TYPE[c.type] || c.type));
      tr.appendChild(el('td', '', ACK_STATUS[c.status] || c.status));
      tr.appendChild(el('td', '', String(c.retry_count)));
      tr.appendChild(el('td', 'mono', fmtTime(c.sent_at)));
      tr.appendChild(el('td', '', c.ack_message || c.last_error || '--'));
      const tdOp = el('td');
      if (opts.showAck && c.status === 'pending') {
        tdOp.appendChild(actionBtn('回执成功', () => API.ackCommand(c.id, { success: true, message: '终端已执行' }), 'secondary'));
        tdOp.appendChild(actionBtn('回执失败', () => API.ackCommand(c.id, { success: false, message: '终端异常' }), 'danger'));
      }
      tr.appendChild(tdOp);
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
    wrap.appendChild(table);
    return wrap;
  }

  function auditTable(items) {
    const wrap = el('div', 'table-wrap');
    if (!items.length) { wrap.appendChild(el('div', 'empty', '暂无审计日志')); return wrap; }
    const table = el('table', 'data');
    table.innerHTML = '<thead><tr><th>时间</th><th>动作</th><th>对象</th><th>操作人</th><th>详情</th></tr></thead>';
    const tbody = el('tbody');
    items.forEach((l) => {
      const tr = el('tr');
      tr.appendChild(el('td', 'mono', fmtTime(l.created_at)));
      tr.appendChild(el('td', '', l.action));
      tr.appendChild(el('td', 'mono', l.entity_type + ':' + l.entity_id));
      tr.appendChild(el('td', '', l.operator || '--'));
      tr.appendChild(el('td', '', l.detail || '--'));
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
    wrap.appendChild(table);
    return wrap;
  }

  /* ============================ 模拟遥测上报 ============================ */
  function telemetrySimulator(id, beacon, latest, chartBody) {
    const panel = el('div', 'panel');
    panel.appendChild(panelHead('📡 模拟遥测上报（触发灯质校验/电压健康/漂移检测）'));
    const body = el('div', 'panel-body');

    const lampSel = el('select');
    [['on', '亮'], ['off', '灭']].forEach(([v, label]) => {
      const opt = el('option', '', label);
      opt.value = v;
      lampSel.appendChild(opt);
    });
    const voltageInput = el('input');
    voltageInput.type = 'number'; voltageInput.step = '0.1'; voltageInput.value = '12.3';
    const currentInput = el('input');
    currentInput.type = 'number'; currentInput.step = '0.1'; currentInput.value = '0.8';
    const latInput = el('input');
    latInput.type = 'number'; latInput.step = '0.0001'; latInput.value = String(beacon.anchor.lat);
    const lngInput = el('input');
    lngInput.type = 'number'; lngInput.step = '0.0001'; lngInput.value = String(beacon.anchor.lng);
    const flashInput = el('input');
    flashInput.type = 'number'; flashInput.step = '0.1'; flashInput.value = String(beacon.lamp_pattern.flash_sec);
    const eclipseInput = el('input');
    eclipseInput.type = 'number'; eclipseInput.step = '0.1'; eclipseInput.value = String(beacon.lamp_pattern.eclipse_sec);

    const row1 = el('div', 'form-row');
    row1.appendChild(field('灯状态', lampSel));
    row1.appendChild(field('电压(V)', voltageInput));
    row1.appendChild(field('电流(A)', currentInput));
    body.appendChild(row1);

    const row2 = el('div', 'form-row');
    row2.appendChild(field('纬度', latInput));
    row2.appendChild(field('经度', lngInput));
    row2.appendChild(field('闪光(s)', flashInput));
    row2.appendChild(field('熄灭(s)', eclipseInput));
    const submit = el('button', 'btn', '上报遥测');
    row2.appendChild(submit);
    body.appendChild(row2);

    const msg = el('div');
    body.appendChild(msg);

    submit.addEventListener('click', async () => {
      submit.disabled = true;
      msg.className = 'msg info';
      msg.textContent = '上报中…';
      const payload = {
        lamp_state: lampSel.value,
        voltage: parseFloat(voltageInput.value) || 0,
        current: parseFloat(currentInput.value) || 0,
        position: { lat: parseFloat(latInput.value) || 0, lng: parseFloat(lngInput.value) || 0 },
      };
      if (lampSel.value === 'on') {
        payload.measured_pattern = {
          flash_sec: parseFloat(flashInput.value) || beacon.lamp_pattern.flash_sec,
          eclipse_sec: parseFloat(eclipseInput.value) || beacon.lamp_pattern.eclipse_sec,
        };
      }
      try {
        const res = await API.reportTelemetry(id, payload);
        msg.className = res.violations && res.violations.length ? 'msg err' : 'msg ok';
        msg.textContent = (res.violations && res.violations.length ? '⚠️ 检测到违规: ' + res.violations.join('；') : '✅ 遥测已入库，无违规') +
          (res.suggested_period ? '；建议遥测周期 ' + res.suggested_period : '');
        // 刷新图表与校验
        const tel = await API.beaconTelemetry(id, 120);
        chartBody.innerHTML = '';
        chartBody.appendChild(TelemetryChart(tel, { title: '近 ' + tel.length + ' 条遥测', lampPattern: BeaconCardUtils.lampPatternText(beacon.lamp_pattern) }));
      } catch (e) {
        msg.className = 'msg err';
        msg.textContent = '❌ 上报失败: ' + e.message;
      } finally {
        submit.disabled = false;
      }
    });

    panel.appendChild(body);
    return panel;
  }

  function field(labelText, input) {
    const f = el('div', 'field');
    f.appendChild(el('label', '', labelText));
    f.appendChild(input);
    return f;
  }

  /* ============================ 启动 ============================ */
  function init() {
    bindNav();
    render();
    setInterval(updateHeaderStatus, 15000);
    updateHeaderStatus();
    updateClock();
    setInterval(updateClock, 1000);
  }

  async function updateHeaderStatus() {
    const elm = document.getElementById('header-status');
    try {
      const h = await API.healthz();
      elm.className = 'header-status ok';
      elm.textContent = '● ' + (h.status || 'ok');
    } catch (e) {
      elm.className = 'header-status err';
      elm.textContent = '● 离线';
    }
  }

  function updateClock() {
    const elm = document.getElementById('footer-clock');
    if (elm) elm.textContent = new Date().toLocaleString('zh-CN');
  }

  function bindNav() {
    document.querySelectorAll('#app-nav a').forEach((a) => {
      a.addEventListener('click', (e) => {
        e.preventDefault();
        navigate(a.getAttribute('href'));
      });
    });
    window.addEventListener('popstate', render);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
