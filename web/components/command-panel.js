/**
 * components/command-panel.js —— CommandPanel 遥控指令面板
 * 被【航标详情】与【遥控记录】共用：选择指令类型（开灯/关灯/切换灯质），
 * 下发后展示回执状态，并支持「模拟终端回执」闭环验证。
 */
(function (global) {
  'use strict';

  const TYPE_LABEL = { on: '开灯', off: '关灯', switch_pattern: '切换灯质' };

  function el(tag, cls, text) {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined && text !== null) node.textContent = text;
    return node;
  }

  /**
   * @param {object} opts
   *  - beaconId: 固定航标 ID（详情页）
   *  - beacons: 航标列表（遥控记录页选择用）
   *  - lampPattern: 当前设定灯质（详情页用于展示）
   * @returns {HTMLElement}
   */
  function CommandPanel(opts) {
    opts = opts || {};
    const panel = el('div', 'panel');
    const head = el('div', 'panel-head');
    head.appendChild(el('h2', '', '🎛️ 遥控指令'));
    panel.appendChild(head);

    const body = el('div', 'panel-body');
    const row = el('div', 'form-row');

    // 航标选择（遥控记录页共用）
    let beaconSelect = null;
    if (!opts.beaconId) {
      beaconSelect = el('select');
      beaconSelect.id = 'cmd-beacon-select';
      (opts.beacons || []).forEach((b) => {
        const opt = el('option', '', b.name + ' (' + b.id + ')');
        opt.value = b.id;
        beaconSelect.appendChild(opt);
      });
      const f = el('div', 'field');
      f.appendChild(el('label', '', '航标'));
      f.appendChild(beaconSelect);
      row.appendChild(f);
    }

    // 指令类型
    const typeSelect = el('select');
    typeSelect.id = 'cmd-type-select';
    Object.keys(TYPE_LABEL).forEach((k) => {
      const opt = el('option', '', TYPE_LABEL[k]);
      opt.value = k;
      typeSelect.appendChild(opt);
    });
    const tf = el('div', 'field');
    tf.appendChild(el('label', '', '指令类型'));
    tf.appendChild(typeSelect);
    row.appendChild(tf);

    // 切换灯质参数
    const flashInput = el('input');
    flashInput.type = 'number';
    flashInput.step = '0.1';
    flashInput.min = '0.1';
    flashInput.value = '2.0';
    const eclipseInput = el('input');
    eclipseInput.type = 'number';
    eclipseInput.step = '0.1';
    eclipseInput.min = '0';
    eclipseInput.value = '2.0';

    const flashField = el('div', 'field');
    flashField.id = 'cmd-flash-field';
    flashField.appendChild(el('label', '', '闪光时长(秒)'));
    flashField.appendChild(flashInput);
    const eclipseField = el('div', 'field');
    eclipseField.id = 'cmd-eclipse-field';
    eclipseField.appendChild(el('label', '', '熄灭时长(秒)'));
    eclipseField.appendChild(eclipseInput);

    if (opts.lampPattern) {
      flashInput.value = String(opts.lampPattern.flash_sec);
      eclipseInput.value = String(opts.lampPattern.eclipse_sec);
    }
    row.appendChild(flashField);
    row.appendChild(eclipseField);

    const submit = el('button', 'btn', '下发指令');
    submit.type = 'button';
    submit.id = 'cmd-submit';
    row.appendChild(submit);
    body.appendChild(row);

    const msg = el('div', '');
    msg.id = 'cmd-msg';
    body.appendChild(msg);

    function setPatternVisible(visible) {
      flashField.style.display = visible ? '' : 'none';
      eclipseField.style.display = visible ? '' : 'none';
    }
    typeSelect.addEventListener('change', () => setPatternVisible(typeSelect.value === 'switch_pattern'));
    setPatternVisible(typeSelect.value === 'switch_pattern');

    async function dispatch() {
      const beaconId = beaconSelect ? beaconSelect.value : opts.beaconId;
      const type = typeSelect.value;
      const payload = { type };
      if (type === 'switch_pattern') {
        payload.target_pattern = {
          flash_sec: parseFloat(flashInput.value) || 2,
          eclipse_sec: parseFloat(eclipseInput.value) || 0,
        };
      }
      submit.disabled = true;
      msg.className = 'msg info';
      msg.textContent = '指令下发中…';
      try {
        const cmd = await API.dispatchCommand(beaconId, payload);
        msg.className = 'msg ok';
        msg.textContent = '✅ 指令 ' + cmd.id + ' 已下发（' + (TYPE_LABEL[cmd.type] || cmd.type) +
          '），等待终端回执，回执期限 ' + fmtTime(cmd.deadline);
        renderAckBar(cmd);
      } catch (e) {
        msg.className = 'msg err';
        msg.textContent = '❌ 下发失败: ' + e.message + (e.status === 409 ? '（漂移守卫拦截）' : '');
      } finally {
        submit.disabled = false;
      }
    }

    function renderAckBar(cmd) {
      const bar = el('div', 'form-row');
      bar.id = 'cmd-ack-bar';
      bar.style.marginTop = '8px';
      const label = el('span', '', '模拟终端回执：');
      bar.appendChild(label);
      const okBtn = el('button', 'btn sm', '回执成功');
      okBtn.type = 'button';
      okBtn.addEventListener('click', async () => {
        try {
          const updated = await API.ackCommand(cmd.id, { success: true, message: '终端已执行' });
          msg.className = 'msg ok';
          msg.textContent = '📨 指令 ' + updated.id + ' 回执成功，状态: ' + updated.status;
          bar.remove();
        } catch (e) {
          msg.className = 'msg err';
          msg.textContent = '❌ 回执失败: ' + e.message;
        }
      });
      const failBtn = el('button', 'btn sm danger', '回执失败');
      failBtn.type = 'button';
      failBtn.addEventListener('click', async () => {
        try {
          const updated = await API.ackCommand(cmd.id, { success: false, message: '终端执行异常' });
          msg.className = 'msg err';
          msg.textContent = '📨 指令 ' + updated.id + ' 回执失败，状态: ' + updated.status;
          bar.remove();
        } catch (e) {
          msg.className = 'msg err';
          msg.textContent = '❌ 回执失败: ' + e.message;
        }
      });
      bar.appendChild(okBtn);
      bar.appendChild(failBtn);
      body.appendChild(bar);
    }

    submit.addEventListener('click', dispatch);
    panel.appendChild(body);
    return panel;
  }

  function fmtTime(iso) {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    const pad = (n) => String(n).padStart(2, '0');
    return pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
  }

  global.CommandPanel = CommandPanel;
})(window);
