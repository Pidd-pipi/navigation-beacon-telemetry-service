/**
 * api.js —— 前端 API 层
 * 统一封装 fetch，解析后端统一响应格式 { code, message, data }，
 * 所有页面与组件只与本模块交互，保证「页面真实消费后端接口」。
 */
(function (global) {
  'use strict';

  function qs(query) {
    if (!query) return '';
    const parts = Object.keys(query)
      .filter((k) => query[k] !== undefined && query[k] !== null && query[k] !== '')
      .map((k) => encodeURIComponent(k) + '=' + encodeURIComponent(query[k]));
    return parts.length ? '?' + parts.join('&') : '';
  }

  async function request(method, url, body) {
    const opts = { method, headers: {} };
    if (body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(body);
    }
    let res;
    try {
      res = await fetch(url, opts);
    } catch (e) {
      throw new Error('网络请求失败: ' + e.message);
    }
    let payload = null;
    try {
      payload = await res.json();
    } catch (e) {
      /* 非 JSON 响应 */
    }
    if (!res.ok) {
      const msg = payload && payload.message ? payload.message : 'HTTP ' + res.status;
      const err = new Error(msg);
      err.status = res.status;
      err.payload = payload;
      throw err;
    }
    return payload ? payload.data : null;
  }

  const API = {
    request,
    get: (url) => request('GET', url),
    post: (url, body) => request('POST', url, body),

    // 健康检查
    healthz: () => request('GET', '/api/healthz'),

    // 总览聚合
    overview: () => request('GET', '/api/overview'),

    // 航标台账
    beacons: () => request('GET', '/api/beacons'),
    createBeacon: (payload) => request('POST', '/api/beacons', payload),
    beacon: (id) => request('GET', '/api/beacons/' + encodeURIComponent(id)),

    // 遥测
    beaconTelemetry: (id, limit) =>
      request('GET', '/api/beacons/' + encodeURIComponent(id) + '/telemetry?limit=' + (limit || 200)),
    reportTelemetry: (id, payload) =>
      request('POST', '/api/beacons/' + encodeURIComponent(id) + '/telemetry', payload),

    // 异常台账
    abnormalities: (query) => request('GET', '/api/abnormalities' + qs(query)),
    createAbnormality: (payload) => request('POST', '/api/abnormalities', payload),
    resolveAbnormality: (id, reason) =>
      request('POST', '/api/abnormalities/' + encodeURIComponent(id) + '/resolve', { reason: reason || '' }),

    // 处置任务
    tasks: (query) => request('GET', '/api/tasks' + qs(query)),
    createTask: (abnormalityId) =>
      request('POST', '/api/tasks', { abnormality_id: abnormalityId }),
    assignTask: (id, assignee) =>
      request('POST', '/api/tasks/' + encodeURIComponent(id) + '/assign', { assignee: assignee || '值班员' }),
    repairTask: (id, note) =>
      request('POST', '/api/tasks/' + encodeURIComponent(id) + '/repair', { note: note || '已完成修复' }),
    verifyTask: (id, result) =>
      request('POST', '/api/tasks/' + encodeURIComponent(id) + '/verify', { result: result || '复测正常', auto_close: true }),
    closeTask: (id) => request('POST', '/api/tasks/' + encodeURIComponent(id) + '/close', {}),
    escalateTask: (id) => request('POST', '/api/tasks/' + encodeURIComponent(id) + '/escalate', {}),

    // 遥控指令
    commands: (query) => request('GET', '/api/commands' + qs(query)),
    dispatchCommand: (beaconId, payload) =>
      request('POST', '/api/beacons/' + encodeURIComponent(beaconId) + '/command', payload),
    ackCommand: (id, payload) =>
      request('POST', '/api/commands/' + encodeURIComponent(id) + '/ack', payload),
    command: (id) => request('GET', '/api/commands/' + encodeURIComponent(id)),

    // 审计日志
    audits: (limit) => request('GET', '/api/audits?limit=' + (limit || 50)),
  };

  global.API = API;
})(window);
