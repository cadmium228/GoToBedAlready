"use strict";

const $ = (id) => document.getElementById(id);
const phases = {
  idle: "Готов к работе",
  waiting: "Ожидание данных",
  downloading: "Загрузка",
  countdown: "Обратный отсчёт",
  cancelled: "Отменено",
  completed: "Готово",
  error: "Ошибка",
};
let selected = null;
let processes = [];
let status = null;
let demo = false;
let initialized = false;
let busy = false;
let connected = false;
let eventsSignature = "";
let sessionToken = "";

try {
  if (location.hash.length > 1) {
    sessionStorage.setItem("gtb-token", location.hash.slice(1));
    history.replaceState(null, "", location.pathname);
  }
  sessionToken = sessionStorage.getItem("gtb-token") || "";
} catch {
  sessionToken = location.hash.slice(1);
}

async function api(path, body) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 20000);
  try {
    const response = await fetch(`/api/${path}`, {
      method: body === undefined ? "GET" : "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Session-Token": sessionToken,
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
      cache: "no-store",
    });
    const result = await response.json();
    if (!response.ok)
      throw new Error(result.error || `Ошибка HTTP ${response.status}`);
    return result;
  } finally {
    clearTimeout(timeout);
  }
}

function showError(error) {
  $("error").textContent = error.message || String(error);
  $("error").hidden = false;
}

function clearError() {
  $("error").hidden = true;
}

function updateSelection() {
  $("selected-name").textContent = selected ? selected.name : "Выбрать процесс";
  $("selected-detail").textContent = selected
    ? `PID ${selected.pid} · ${selected.title}`
    : "Steam, Rockstar или другой лаунчер";
}

function updateControls() {
  const active = Boolean(status?.active);
  for (const id of ["choose-process", "delay", "minus", "plus"])
    $(id).disabled = active || busy || !connected;
  $("dry-run").disabled = active || busy || demo || !connected;
  $("start").hidden = active;
  $("start").disabled = busy || !connected;
  $("cancel").hidden = !active;
  $("cancel").disabled = busy || !connected;
  const dryRun = active ? status.preferences.dryRun : $("dry-run").checked;
  const delay = active
    ? status.preferences.delayMinutes
    : Number($("delay").value);
  $("delay-display").textContent = `${delay || "—"} мин`;
  $("action-display").textContent = dryRun ? "Тестовый режим" : "Выключение ПК";
  $("mode-hint").textContent = dryRun
    ? "Безопасный старт: ПК останется включённым"
    : "Компьютер выключится после завершения загрузки";
}

function render(next) {
  status = next;
  $("phase").textContent = phases[status.phase] || status.phase;
  $("phase").classList.toggle("active", status.active);
  $("status-message").textContent = status.message;
  const hasProgress = status.progress !== null;
  if (hasProgress) {
    $("progress-text").textContent = `${Math.floor(status.progress)}%`;
  } else {
    $("progress-text").textContent = status.active ? "—" : "zzz";
  }
  $("sleep-icon").toggleAttribute("hidden", hasProgress);
  $("progress-label").textContent = hasProgress
    ? "Прогресс загрузки"
    : status.active
      ? "Нет достоверных данных"
      : "Всё под контролем";
  $("ring-value").style.strokeDashoffset = hasProgress
    ? String(100 - status.progress)
    : "100";
  const captions = {
    idle: "Вы отдыхаете. Мы следим.",
    waiting: "Проверяем, не торопимся.",
    downloading: "Игра скоро будет готова.",
    countdown: "Последние минуты до сна.",
    cancelled: "Вы всё контролируете.",
    completed: "Можно спать спокойно.",
    error: "Нужно ваше внимание.",
  };
  $("status-caption").textContent = captions[status.phase] || "Наблюдение";
  $("connection-dot").classList.toggle(
    "connected",
    hasProgress && status.active,
  );
  $("source-label").textContent = status.active
    ? hasProgress
      ? `Источник: ${status.source.startsWith("taskbar") ? "панель задач Windows" : status.source.startsWith("demo") ? "демонстрация" : "UI Automation окна"}`
      : "Нет доступного индикатора · выключение заблокировано"
    : "Наблюдение не запущено";
  $("storage-warning").hidden = !status.storageWarning;
  $("storage-warning").textContent = status.storageWarning;
  $("countdown-box").hidden = !status.deadline || !status.active;
  updateCountdown();
  const signature = JSON.stringify(status.events);
  if (signature !== eventsSignature) {
    eventsSignature = signature;
    $("events").replaceChildren();
    if (!status.events.length) {
      const item = document.createElement("li");
      item.className = "empty-event";
      item.textContent = "Пока тихо. Самое время начать.";
      $("events").append(item);
    }
    for (const event of [...status.events].reverse()) {
      const item = document.createElement("li");
      const time = document.createElement("time");
      time.dateTime = event.time;
      time.title = new Date(event.time).toLocaleString("ru-RU");
      time.textContent = new Date(event.time).toLocaleTimeString("ru-RU", {
        hour: "2-digit",
        minute: "2-digit",
      });
      const message = document.createElement("span");
      message.textContent = event.message;
      item.append(time, message);
      $("events").append(item);
    }
  }
  if (status.active) {
    selected = status.process;
    $("delay").value = status.preferences.delayMinutes;
    $("dry-run").checked = status.preferences.dryRun;
    updateSelection();
  }
  updateControls();
}

function updateCountdown() {
  if (!status?.deadline) return;
  if (!connected) {
    $("countdown").textContent = "Нет связи";
    return;
  }
  const seconds = Math.max(
    0,
    Math.ceil((new Date(status.deadline).getTime() - Date.now()) / 1000),
  );
  $("countdown").textContent =
    seconds === 0
      ? "Проверяем…"
      : `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

async function poll() {
  try {
    const result = await api("status");
    connected = true;
    demo = result.demo;
    $("demo-banner").hidden = !demo;
    $("version").textContent = result.version;
    $("environment").textContent = demo
      ? "Безопасная демонстрация"
      : "Локально на вашем ПК";
    if (!initialized) {
      $("delay").value = result.status.preferences.delayMinutes;
      $("dry-run").checked = demo || result.status.preferences.dryRun;
      initialized = true;
    }
    document.querySelector(".local-badge .dot").classList.remove("offline");
    render(result.status);
  } catch (error) {
    connected = false;
    document.querySelector(".local-badge .dot").classList.add("offline");
    $("environment").textContent = "Нет связи с приложением";
    $("status-message").textContent =
      "Связь потеряна. Сервер мог продолжить наблюдение. Для гарантированной отмены нажмите Ctrl+C в консоли.";
    $("source-label").textContent = "Данные устарели · проверьте консоль";
    $("connection-dot").classList.remove("connected");
    showError(error);
    updateControls();
  } finally {
    setTimeout(poll, 1500);
  }
}

function renderProcesses() {
  const query = $("process-search").value.toLocaleLowerCase();
  const filtered = processes.filter((process) =>
    `${process.name} ${process.title} ${process.pid}`
      .toLocaleLowerCase()
      .includes(query),
  );
  $("process-list").replaceChildren();
  if (!filtered.length) {
    const message = document.createElement("p");
    message.className = "list-message";
    message.textContent = processes.length
      ? "Ничего не найдено. Попробуйте другое название."
      : "Окон не найдено. Откройте лаунчер и обновите список.";
    $("process-list").append(message);
  }
  for (const process of filtered) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "process-option";
    const copy = document.createElement("span");
    const name = document.createElement("strong");
    name.textContent = process.name;
    const title = document.createElement("small");
    title.textContent = process.title;
    const pid = document.createElement("span");
    pid.textContent = `PID ${process.pid}`;
    copy.append(name, title);
    button.append(copy, pid);
    button.addEventListener("click", () => {
      selected = process;
      updateSelection();
      clearError();
      $("process-dialog").close();
    });
    $("process-list").append(button);
  }
}

async function loadProcesses() {
  $("refresh-processes").disabled = true;
  $("process-list").textContent = "Получаем список окон…";
  try {
    processes = await api("processes");
    renderProcesses();
  } catch (error) {
    $("process-list").textContent = error.message;
  } finally {
    $("refresh-processes").disabled = false;
  }
}

async function start() {
  if (busy) return;
  busy = true;
  clearError();
  updateControls();
  try {
    render(
      await api("start", {
        process: selected,
        preferences: {
          delayMinutes: Number($("delay").value),
          dryRun: $("dry-run").checked,
        },
      }),
    );
  } catch (error) {
    showError(error);
  } finally {
    busy = false;
    updateControls();
  }
}

$("choose-process").addEventListener("click", () => {
  $("process-search").value = "";
  $("process-dialog").showModal();
  loadProcesses();
});
$("close-dialog").addEventListener("click", () => $("process-dialog").close());
$("refresh-processes").addEventListener("click", loadProcesses);
$("process-search").addEventListener("input", renderProcesses);
$("dry-run").addEventListener("change", updateControls);
$("delay").addEventListener("input", updateControls);
for (const [id, change] of [
  ["minus", -1],
  ["plus", 1],
]) {
  $(id).addEventListener("click", () => {
    $("delay").value = Math.min(
      180,
      Math.max(1, (Number($("delay").value) || 5) + change),
    );
    updateControls();
  });
}
$("setup-form").addEventListener("submit", (event) => {
  event.preventDefault();
  if (!selected) {
    showError(new Error("Сначала выберите процесс с загрузкой."));
    $("choose-process").focus();
    return;
  }
  if (!$("dry-run").checked && !demo) {
    $("confirm-dialog").showModal();
  } else {
    start();
  }
});
$("confirm-back").addEventListener("click", () => $("confirm-dialog").close());
$("confirm-start").addEventListener("click", () => {
  $("confirm-dialog").close();
  start();
});
$("cancel").addEventListener("click", async () => {
  busy = true;
  updateControls();
  try {
    render(await api("cancel", {}));
    clearError();
  } catch (error) {
    showError(error);
  } finally {
    busy = false;
    updateControls();
  }
});
updateControls();
setInterval(updateCountdown, 500);
poll();
