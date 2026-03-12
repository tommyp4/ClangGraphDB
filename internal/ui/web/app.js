import { nodes, links, nodesMap, linksMap } from './js/state.js';
import { fetchOverview } from './js/api.js';
import { initGraph, updateGraph, renderGraph } from './js/graph.js';
import { updateLegend } from './js/ui.js';
import { initEventListeners, getHandlers } from './js/interactions.js';

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

    // Initial render and data fetch
    updateLegend();
    
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
