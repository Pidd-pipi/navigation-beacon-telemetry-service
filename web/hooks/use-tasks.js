/**
 * hooks/use-tasks.js —— useTasks(filter, 轮询)
 * 处置任务数据 Hook，供【处置任务】与【航标总览】共用：
 * 支持按状态/航标过滤、定时轮询、手动刷新。
 */
(function (global) {
  'use strict';

  /**
   * @param {object} filter 过滤条件 { status, beacon_id, level }
   * @param {number} pollMs 轮询间隔（毫秒），<=0 表示不轮询
   * @param {Function} onChange 状态变更回调 (state) => void
   */
  function useTasks(filter, pollMs, onChange) {
    let timer = null;
    let currentFilter = filter || {};
    let state = { tasks: [], loading: true, error: null };

    function emit() {
      if (typeof onChange === 'function') onChange(state);
      return state;
    }

    async function refresh() {
      try {
        const tasks = await API.tasks(currentFilter);
        state = { tasks: tasks || [], loading: false, error: null };
      } catch (e) {
        state = { ...state, loading: false, error: e.message };
      }
      return emit();
    }

    function setFilter(next) {
      currentFilter = next || {};
      return refresh();
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
      setFilter,
      get state() {
        return state;
      },
    };
  }

  global.useTasks = useTasks;
})(window);
