import { nodes, links, nodesMap, linksMap } from './js/state.js';
import { fetchOverview, fetchStatus, fetchConfig } from './js/api.js';
import { initGraph, updateGraph, renderGraph } from './js/graph.js';
import { updateLegend } from './js/ui.js';
import { initEventListeners, getHandlers } from './js/interactions.js';

async function pollStatus() {
    try {
        const status = await fetchStatus();
        const nodesCount = status.stats && status.stats.nodes ? status.stats.nodes.toLocaleString() : '0';
        document.getElementById('status-indexed').textContent = `INDEXED: ${nodesCount} NODES`;
        
        const indicator = document.getElementById('status-indicator');
        const text = document.getElementById('status-text');
        
        indicator.className = 'size-1.5 rounded-full bg-green-500 animate-pulse';
        text.textContent = 'LIVE SYNC';
    } catch (err) {
        document.getElementById('status-indicator').className = 'size-1.5 rounded-full bg-red-500';
        document.getElementById('status-text').textContent = 'OFFLINE';
    }
    setTimeout(pollStatus, 5000);
}

function initSettingsModal() {
    const btnSettings = document.getElementById('btn-settings');
    const modal = document.getElementById('settings-modal');
    const btnClose = document.getElementById('btn-close-settings');
    const btnOk = document.getElementById('btn-settings-ok');
    const btnTogglePassword = document.getElementById('btn-toggle-password');
    const iconTogglePassword = document.getElementById('icon-toggle-password');
    const elPassword = document.getElementById('cfg-neo4j-password');

    const showModal = async () => {
        try {
            const cfg = await fetchConfig();
            document.getElementById('cfg-neo4j-uri').textContent = cfg.Neo4jURI || 'N/A';
            document.getElementById('cfg-neo4j-user').textContent = cfg.Neo4jUser || 'N/A';
            
            // Set up masked password initially
            if (elPassword) {
                elPassword.dataset.password = cfg.Neo4jPassword || '';
                elPassword.textContent = '******';
            }
            if (iconTogglePassword) {
                iconTogglePassword.textContent = 'visibility_off';
            }

            document.getElementById('cfg-gcp-project').textContent = cfg.GoogleCloudProject || 'N/A';
            document.getElementById('cfg-embedding-model').textContent = cfg.GeminiEmbeddingModel || 'N/A';
            document.getElementById('cfg-generative-model').textContent = cfg.GeminiGenerativeModel || 'N/A';
            
            modal.classList.remove('hidden');
            modal.classList.add('flex');
        } catch (err) {
            console.error("Failed to load config:", err);
            alert("Failed to load configuration.");
        }
    };

    const hideModal = () => {
        modal.classList.add('hidden');
        modal.classList.remove('flex');
    };

    const togglePassword = () => {
        if (!elPassword || !iconTogglePassword) return;
        const isHidden = iconTogglePassword.textContent === 'visibility_off';
        
        if (isHidden) {
            elPassword.textContent = elPassword.dataset.password || '';
            iconTogglePassword.textContent = 'visibility';
        } else {
            elPassword.textContent = '******';
            iconTogglePassword.textContent = 'visibility_off';
        }
    };

    if (btnSettings) btnSettings.addEventListener('click', showModal);
    if (btnClose) btnClose.addEventListener('click', hideModal);
    if (btnOk) btnOk.addEventListener('click', hideModal);
    if (btnTogglePassword) btnTogglePassword.addEventListener('click', togglePassword);
}

async function bootstrap() {
    console.log("GraphDB Visualizer bootstrapping...");

    // Initialize the graph engine
    const handlers = getHandlers();
    initGraph(
        handlers.handleNodeClick,
        handlers.handleNodeMouseOver,
        handlers.handleNodeMouseOut,
        handlers.handleNodeDoubleClick,
        handlers.handleNodeContextMenu
    );

    // Bind UI events
    initEventListeners();
    initSettingsModal();

    // Initial render and data fetch
    updateLegend();
    pollStatus();
    
    try {
        const data = await fetchOverview();
        if (data && data.nodes) {
            const fetchedNodes = data.nodes.map(n => ({
                id: n.id || n.Id,
                name: (n.properties && n.properties.name) || (n.Properties && n.Properties.name) || n.id || n.Id,
                properties: n.properties || n.Properties,
                ...n
            }));
            
            nodesMap.clear();
            linksMap.clear();
            nodes.length = 0;
            links.length = 0;
            updateGraph(fetchedNodes, data.edges || []);
        }
    } catch (err) {
        console.error("Failed to fetch initial overview:", err);
    }

    console.log("GraphDB Visualizer initialized. Version: " + new Date().toISOString());
}

// Start the application
bootstrap();
