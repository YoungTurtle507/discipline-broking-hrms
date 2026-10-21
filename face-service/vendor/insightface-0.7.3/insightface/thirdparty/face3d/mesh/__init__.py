#from __future__ import absolute_import
#from cython import mesh_core_cython
#import io
#import vis
#import transform
#import light
#import render

# mesh_core_cython is a compiled C++ extension for 3D face reconstruction
# that this project doesn't build (no C++ compiler available, and it's only
# needed for mask_renderer / 3D swapping, not 2D detection+embedding). io,
# light, and render all depend on it at import time, so guard everything
# that touches it and let the rest of the package load without it.
try:
    from .cython import mesh_core_cython
    from . import io
    from . import light
    from . import render
except ImportError:
    mesh_core_cython = None
    io = None
    light = None
    render = None
from . import vis
from . import transform

