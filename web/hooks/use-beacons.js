/**
 * hooks/use-beacons.js —— useBeacons(轮询)
 * 航标列表数据 Hook，供【航标总览】与【航标详情】共用：
 * 支持定时轮询、手动刷新，并通过 onChange 回调把最新状态推给页面。
 */
(function (global) {
  'use strict';

  /**
   * @param {number} pollMs 轮询间隔（毫秒），<=0 表示不轮询
   * @param {Function} onChange 状态变更回调 (state) => void
   * @returns {{ start: Function, stop: Function, refresh: Function, state: object }}
   */
  function useBeacons(pollMs, onChange) {
    let timer = null;
    let state = { beacons: [], loading: true, error: null };

    function emit() {
      if (typeof onChange === 'function') onChange(state);
      return state;
    }

    async function refresh() {
      try {
        const beacons = await API.beacons();
        state = { beacons: beacons || [], loading: false, error: null };
      } catch (e) {
        state = { ...state, loading: false, error: e.message };
      }
      return emit();
    }

    function start() {
      refresh();
      if (pollMs && pollMs > 0 && !timer) {
        timer = setInterval(refresh, pollMs);
      }
      return stop;
    }

    function stop() {
      if (timer) {
        clearInterval(timer);
        timer = null;
      }
    }

    return {
      start,
      stop,
      refresh,
      get state() {
        return state;
      },
    };
  }

  global.useBeacons = useBeacons;
})(window);
