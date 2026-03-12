export const nodes = [];
export const links = [];
export const nodesMap = new Map();
export const linksMap = new Map();

export const state = {
    lastSelectedNode: null,
    labelsVisible: true
};

export const visibilitySettings = {
    showPhysical: true,
    showSemantic: true
};

export const seamState = {
    showPinchPoints: false,
    showSemanticSeams: false,
    pinchPoints: new Set(),
    semanticSeams: []
};
