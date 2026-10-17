"use strict";

const $ = (id) => document.getElementById(id);
let token = location.hash.slice(1);
try {
  if (token) sessionStorage.setItem("gtb-token", token);
  token = token || sessionStorage.getItem("gtb-token") || "";
  history.replaceState(null, "", location.pathname);
} catch {}
let selected = null;
let processes = [];
let state = null;
let demo = false;
let connected = false;
let busy = false;
let savingPower = false;
let initialized = false;
let networkFailed = false;

async function api(path, body) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 20000);
  try {
    const response = await fetch(`/api/${path}`, {
      method: body === undefined ? "GET" : "POST",
      headers: { "Content-Type": "application/json", "X-Session-Token": token },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
      cache: "no-store",
    });
    const result = await response.json();
    if (!response.ok)
      throw new Error(result.error || `Ошибка ${response.status}`);
    return result;
  } finally {
    clearTimeout(timer);
  }
}

function error(message) {
  $("error").textContent = message;
  $("error").hidden = !message;
}

function selection() {
  $("selected-name").textContent = selected ? selected.name : "Выбрать процесс";
  $("selected-detail").hidden = !selected;
  $("selected-detail").textContent = selected
    ? `PID ${selected.pid}${selected.title ? " · " + selected.title : ""}`
    : "";
}

function controls() {
  const active = Boolean(state?.active);
  for (const id of ["choose-process", "delay", "minus", "plus"])
    $(id).disabled = !connected || busy || active;
  for (const id of ["blank-display", "prevent-sleep"])
    $(id).disabled = !connected || savingPower;
  $("start").hidden = active;
  $("cancel").hidden = !active;
  $("start").disabled = !connected || busy || savingPower || !selected;
  $("cancel").disabled = !connected || busy;
  $("diagnostics").hidden =
    demo || !selected || state?.phase !== "waiting" || !state?.active;
  $("diagnostics").disabled = busy || !connected;
}

function render(status) {
  state = status;
  if (status.active) {
    selected = status.process;
    selection();
    $("delay").value = status.preferences.delayMinutes;
  }
  if (!initialized || status.active)
    $("delay").value = status.preferences.delayMinutes;
  if (!savingPower) {
    $("blank-display").checked = status.preferences.blankDisplay;
    $("prevent-sleep").checked = status.preferences.preventSleep;
  }
  initialized = true;
  $("progress-section").hidden = !status.active;
  if (status.progress === null) $("progress-bar").removeAttribute("value");
  else $("progress-bar").value = status.progress;
  $("progress-value").textContent =
    status.progress === null ? "—" : `${Math.floor(status.progress)}%`;
  $("progress-caption").textContent = "Загрузка";
  const warning =
    status.active && status.phase === "waiting"
      ? status.message.replace(
          "Прогресс недоступен. Выключение заблокировано: ",
          "",
        )
      : status.phase === "error"
        ? status.message
        : "";
  $("observation-warning").hidden = !warning;
  $("observation-warning").textContent = warning;
  countdown();
  controls();
}

function countdown() {
  if (!state?.active || !state.deadline || !connected) return;
  const seconds = Math.max(
    0,
    Math.ceil((new Date(state.deadline) - Date.now()) / 1000),
  );
  $("progress-caption").textContent = demo
    ? "До завершения"
    : "Выключение через";
  $("progress-value").textContent = seconds
    ? `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`
    : "Проверка…";
}

async function poll() {
  try {
    const result = await api("status");
    if (networkFailed) error("");
    networkFailed = false;
    connected = true;
    demo = result.demo;
    $("preview-label").hidden = !demo;
    const warning = result.power.warning || result.status.storageWarning;
    $("power-warning").hidden = !warning;
    $("power-warning").textContent = warning;
    render(result.status);
  } catch (failure) {
    connected = false;
    networkFailed = true;
    error(
      "Нет связи с приложением. Закройте его окно, чтобы остановить наблюдение.",
    );
    $("progress-section").hidden = true;
    controls();
  } finally {
    setTimeout(poll, 1000);
  }
}

function renderProcesses() {
  const query = $("process-search").value.toLocaleLowerCase();
  $("process-list").replaceChildren();
  const matches = processes.filter((process) =>
    `${process.name} ${process.title} ${process.pid}`
      .toLocaleLowerCase()
      .includes(query),
  );
  if (!matches.length) $("process-list").textContent = "Ничего не найдено.";
  for (const process of matches) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "process-option";
    const description = document.createElement("span");
    const name = document.createElement("strong");
    const title = document.createElement("small");
    const pid = document.createElement("span");
    name.textContent = process.name;
    title.textContent = process.title || "Без отдельного окна";
    pid.textContent = `PID ${process.pid}`;
    description.append(name, title);
    button.append(description, pid);
    button.addEventListener("click", () => {
      selected = process;
      selection();
      controls();
      error("");
      $("process-dialog").close();
    });
    $("process-list").append(button);
  }
}

async function loadProcesses() {
  $("refresh-processes").disabled = true;
  $("process-list").textContent = "Получение процессов…";
  try {
    processes = await api("processes");
    renderProcesses();
  } catch (failure) {
    $("process-list").textContent = failure.message;
  } finally {
    $("refresh-processes").disabled = false;
  }
}

async function start() {
  busy = true;
  controls();
  error("");
  try {
    render(
      await api("start", {
        process: selected,
        preferences: { delayMinutes: Number($("delay").value), dryRun: demo },
      }),
    );
  } catch (failure) {
    error(failure.message);
  } finally {
    busy = false;
    controls();
  }
}

async function savePower() {
  savingPower = true;
  controls();
  error("");
  try {
    await api("power", {
      blankDisplay: $("blank-display").checked,
      preventSleep: $("prevent-sleep").checked,
    });
  } catch (failure) {
    $("blank-display").checked = state.preferences.blankDisplay;
    $("prevent-sleep").checked = state.preferences.preventSleep;
    error(failure.message);
  } finally {
    savingPower = false;
    controls();
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
for (const id of ["blank-display", "prevent-sleep"])
  $(id).addEventListener("change", savePower);
for (const [id, amount] of [
  ["minus", -1],
  ["plus", 1],
])
  $(id).addEventListener("click", () => {
    $("delay").value = Math.max(
      1,
      Math.min(180, (Number($("delay").value) || 5) + amount),
    );
  });
$("setup-form").addEventListener("submit", (event) => {
  event.preventDefault();
  if (!selected || busy || savingPower) return;
  if (demo) {
    start();
    return;
  }
  $("confirm-copy").textContent =
    `ПК выключится после подтверждённых 100% и задержки ${$("delay").value} мин.`;
  $("confirm-dialog").showModal();
});
$("confirm-back").addEventListener("click", () => $("confirm-dialog").close());
$("confirm-start").addEventListener("click", () => {
  $("confirm-dialog").close();
  start();
});
$("cancel").addEventListener("click", async () => {
  busy = true;
  controls();
  try {
    render(await api("cancel", {}));
    error("");
  } catch (failure) {
    error(failure.message);
  } finally {
    busy = false;
    controls();
  }
});
$("diagnostics").addEventListener("click", () =>
  $("diagnostic-dialog").showModal(),
);
$("diagnostic-back").addEventListener("click", () =>
  $("diagnostic-dialog").close(),
);
$("diagnostic-save").addEventListener("click", async () => {
  $("diagnostic-dialog").close();
  busy = true;
  controls();
  try {
    const report = await api("diagnostics", selected);
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(report, null, 2)], { type: "application/json" }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "GoToBedAlready-diagnostics.json";
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 30000);
  } catch (failure) {
    error(failure.message);
  } finally {
    busy = false;
    controls();
  }
});
controls();
setInterval(countdown, 500);
poll();
