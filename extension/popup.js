function updateStatus() {
  chrome.runtime.sendMessage({ action: "getStatus" }, (response) => {
    if (chrome.runtime.lastError) {
      console.error(chrome.runtime.lastError);
      return;
    }
    
    const statusBadge = document.getElementById('status-badge');
    const statusText = document.getElementById('status-text');
    const hostName = document.getElementById('host-name');

    if (response.connected) {
      statusBadge.classList.add('online');
      statusBadge.classList.remove('offline');
      statusText.textContent = 'Conectado ao Go';
    } else {
      statusBadge.classList.add('offline');
      statusBadge.classList.remove('online');
      statusText.textContent = 'Desconectado';
    }
    hostName.textContent = response.hostName || "Nenhum";
  });
}

// Inicializa e configura listeners
document.addEventListener('DOMContentLoaded', () => {
  updateStatus();
  setInterval(updateStatus, 1500);

  document.getElementById('refresh-btn').addEventListener('click', () => {
    const btn = document.getElementById('refresh-btn');
    btn.textContent = 'Atualizando...';
    updateStatus();
    setTimeout(() => { btn.textContent = 'Atualizar'; }, 600);
  });

  document.getElementById('options-btn').addEventListener('click', () => {
    if (chrome.runtime.openOptionsPage) {
      chrome.runtime.openOptionsPage();
    } else {
      window.open(chrome.runtime.getURL('options.html'));
    }
  });
});

