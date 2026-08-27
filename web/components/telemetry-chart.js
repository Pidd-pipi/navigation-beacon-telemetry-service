/**
 * components/telemetry-chart.js —— TelemetryChart 遥测趋势图
 * 纯 SVG 折线图，展示电压趋势与灯状态（灭灯点标红），
 * 被【航标详情】与【异常台账】共用。
 */
(function (global) {
  'use strict';

  const W = 640;
  const H = 220;
  const PAD = { top: 18, right: 16, bottom: 28, left: 46 };

  function fmtHm(iso) {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return '';
    const pad = (n) => String(n).padStart(2, '0');
    return pad(d.getHours()) + ':' + pad(d.getMinutes());
  }

  /**
   * @param {Array} points 遥测点 [{reported_at, voltage, lamp_state, violations}]
   * @param {object} opts { title, lampPattern }
   * @returns {HTMLElement} 图表容器
   */
  function TelemetryChart(points, opts) {
    opts = opts || {};
    const wrap = document.createElement('div');
    wrap.className = 'chart-wrap';

    const title = document.createElement('div');
    title.style.cssText = 'font-size:13px;color:var(--text-dim);margin-bottom:6px;';
    title.textContent = opts.title || '遥测趋势';
    if (opts.lampPattern) {
      title.textContent += ' · 设定灯质 ' + opts.lampPattern;
    }
    wrap.appendChild(title);

    const pts = (points || []).slice().sort((a, b) => new Date(a.reported_at) - new Date(b.reported_at));
    if (pts.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'empty';
      empty.textContent = '暂无遥测数据';
      wrap.appendChild(empty);
      return wrap;
    }

    // 电压范围
    let minV = Infinity;
    let maxV = -Infinity;
    pts.forEach((p) => {
      if (p.voltage < minV) minV = p.voltage;
      if (p.voltage > maxV) maxV = p.voltage;
    });
    if (!isFinite(minV) || minV === maxV) {
      minV = (minV === Infinity ? 0 : minV) - 0.5;
      maxV = maxV === -Infinity ? 14 : maxV + 0.5;
    } else {
      const padV = Math.max((maxV - minV) * 0.2, 0.3);
      minV -= padV;
      maxV += padV;
    }

    const t0 = new Date(pts[0].reported_at).getTime();
    const t1 = new Date(pts[pts.length - 1].reported_at).getTime();
    const span = Math.max(t1 - t0, 1);

    const x = (t) => PAD.left + ((t - t0) / span) * (W - PAD.left - PAD.right);
    const y = (v) => PAD.top + ((maxV - v) / (maxV - minV)) * (H - PAD.top - PAD.bottom);

    const svgNS = 'http://www.w3.org/2000/svg';
    const svg = document.createElementNS(svgNS, 'svg');
    svg.setAttribute('viewBox', '0 0 ' + W + ' ' + H);

    // 网格 + Y 轴刻度
    const yTicks = 5;
    for (let i = 0; i <= yTicks; i++) {
      const v = maxV - ((maxV - minV) * i) / yTicks;
      const yy = y(v);
      const line = document.createElementNS(svgNS, 'line');
      line.setAttribute('x1', PAD.left);
      line.setAttribute('x2', W - PAD.right);
      line.setAttribute('y1', yy);
      line.setAttribute('y2', yy);
      line.setAttribute('stroke', 'rgba(255,255,255,0.08)');
      line.setAttribute('stroke-dasharray', '3 3');
      svg.appendChild(line);
      const label = document.createElementNS(svgNS, 'text');
      label.setAttribute('x', PAD.left - 6);
      label.setAttribute('y', yy + 4);
      label.setAttribute('text-anchor', 'end');
      label.setAttribute('fill', 'var(--text-dim)');
      label.setAttribute('font-size', '10');
      label.textContent = v.toFixed(1);
      svg.appendChild(label);
    }

    // X 轴时间刻度（5 个）
    const xTicks = 5;
    for (let i = 0; i < xTicks; i++) {
      const t = t0 + (span * i) / (xTicks - 1);
      const xx = x(t);
      const label = document.createElementNS(svgNS, 'text');
      label.setAttribute('x', xx);
      label.setAttribute('y', H - 8);
      label.setAttribute('text-anchor', 'middle');
      label.setAttribute('fill', 'var(--text-dim)');
      label.setAttribute('font-size', '10');
      label.textContent = fmtHm(new Date(t));
      svg.appendChild(label);
    }

    // 电压折线
    const polyline = document.createElementNS(svgNS, 'polyline');
    const linePts = pts.map((p) => x(new Date(p.reported_at).getTime()) + ',' + y(p.voltage)).join(' ');
    polyline.setAttribute('points', linePts);
    polyline.setAttribute('fill', 'none');
    polyline.setAttribute('stroke', 'var(--accent)');
    polyline.setAttribute('stroke-width', '2');
    svg.appendChild(polyline);

    // 数据点：绿=灯亮，红=灭灯
    pts.forEach((p) => {
      const cx = x(new Date(p.reported_at).getTime());
      const cy = y(p.voltage);
      const circle = document.createElementNS(svgNS, 'circle');
      circle.setAttribute('cx', cx);
      circle.setAttribute('cy', cy);
      circle.setAttribute('r', '4');
      const off = p.lamp_state === 'off';
      circle.setAttribute('fill', off ? 'var(--danger)' : 'var(--ok)');
      if (p.violations && p.violations.length) {
        circle.setAttribute('stroke', 'var(--warn)');
        circle.setAttribute('stroke-width', '2');
      }
      const tip = document.createElementNS(svgNS, 'title');
      tip.textContent = fmtHm(p.reported_at) + ' 电压 ' + p.voltage.toFixed(2) + 'V 灯' +
        (off ? '灭' : '亮') + (p.violations && p.violations.length ? ' 违规:' + p.violations.join(';') : '');
      circle.appendChild(tip);
      svg.appendChild(circle);
    });

    wrap.appendChild(svg);

    // 图例
    const legend = document.createElement('div');
    legend.className = 'legend';
    legend.innerHTML =
      '<span><span class="dot" style="background:var(--ok)"></span>灯亮</span>' +
      '<span><span class="dot" style="background:var(--danger)"></span>灭灯</span>' +
      '<span><span class="dot" style="background:var(--warn);border:2px solid var(--warn)"></span>含违规项</span>';
    wrap.appendChild(legend);

    return wrap;
  }

  global.TelemetryChart = TelemetryChart;
})(window);
