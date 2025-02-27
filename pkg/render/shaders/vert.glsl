#version 460 core
layout (location = 0) in uint a_packedVertex;

uniform mat4 model;
uniform mat4 view;
uniform mat4 projection;

// Add buffer for chunk positions (one entry per chunk)
// This will be indexed by gl_DrawID when using MultiDrawElementsIndirect
layout(std430, binding = 0) readonly buffer ChunkPositions {
    vec4 chunkPositionsBuffer[];
};

out vec3 Normal;
out vec3 FragPos;
out vec3 Color;

// Lookup tables for face normals based on orientation
// Updated to match the swapped X and Z coordinate system
const vec3 NORMALS[6] = vec3[6](
    vec3(-1, 0, 0),  // North (originally -Z, now -X)
    vec3(1, 0, 0),   // South (originally +Z, now +X)
    vec3(0, 0, 1),   // East (originally +X, now +Z)
    vec3(0, 0, -1),  // West (originally -X, now -Z)
    vec3(0, 1, 0),   // Up (unchanged)
    vec3(0, -1, 0)   // Down (unchanged)
);

// Lookup table for block colors based on texture ID
// In a real implementation, you'd use a texture atlas instead
const vec3 BLOCK_COLORS[18] = vec3[18](
    vec3(1.0, 1.0, 1.0),   // Air (white, should never be rendered)
    vec3(0.2, 0.8, 0.2),   // Grass
    vec3(0.6, 0.4, 0.2),   // Dirt
    vec3(0.5, 0.5, 0.5),   // Stone
    vec3(0.6, 0.3, 0.0),   // OakLog
    vec3(0.2, 0.6, 0.0),   // OakLeaves
    vec3(0.9, 0.9, 1.0),   // Glass
    vec3(0.0, 0.4, 0.8),   // Water
    vec3(0.9, 0.9, 0.6),   // Sand
    vec3(0.9, 0.9, 0.9),   // Snow
    vec3(0.8, 0.5, 0.3),   // OakPlanks
    vec3(0.4, 0.4, 0.4),   // StoneBricks
    vec3(0.4, 0.0, 0.0),   // Netherrack
    vec3(1.0, 0.8, 0.0),   // GoldBlock
    vec3(0.8, 0.9, 1.0),   // PackedIce
    vec3(1.0, 0.3, 0.0),   // Lava
    vec3(0.5, 0.3, 0.1),   // Barrel
    vec3(0.4, 0.2, 0.0)    // Bookshelf
);

// Function to apply ambient occlusion factor
float getAmbientOcclusionFactor(uint aoValue) {
    // Convert 3-bit AO value (0-7) to a float factor (0.4-1.0)
    return 0.4 + (float(aoValue) / 7.0) * 0.6;
}

void main()
{
    // Unpack vertex data
    // Note: X and Z are swapped in the packed format to match server coordinates
    int a_z =                   int((a_packedVertex >> 0)  & 31);  // This is actually the Z coordinate
    int a_y =                   int((a_packedVertex >> 5)  & 31);
    int a_x =                   int((a_packedVertex >> 10) & 31);  // This is actually the X coordinate
    int a_u =                   int((a_packedVertex >> 15) & 1);
    int a_v =                   int((a_packedVertex >> 16) & 1);
    uint a_orientation =           ((a_packedVertex >> 17) & 7);
    uint a_texture_id =            ((a_packedVertex >> 20) & 255);
    uint a_ambient_occlusion =     ((a_packedVertex >> 28) & 7);
    
    // Get chunk-specific position from the buffer using gl_DrawID
    // This is the magic that makes multi-draw indirect work properly
    vec3 currentChunkPosition;
    
    // When using MultiDrawElementsIndirect, gl_DrawID contains the current draw command index
    if (gl_DrawID < chunkPositionsBuffer.length()) {
        currentChunkPosition = chunkPositionsBuffer[gl_DrawID].xyz;
    }
    
    // Get position in world space by combining local vertex position with chunk position
    vec3 position = vec3(a_x, a_y, a_z) + currentChunkPosition;
    
    // Get normal from orientation
    vec3 normal = NORMALS[a_orientation];
    
    // Get base color from texture ID
    vec3 baseColor = BLOCK_COLORS[min(a_texture_id, 17u)];
    
    // Apply ambient occlusion
    float aoFactor = getAmbientOcclusionFactor(a_ambient_occlusion);
    vec3 color = baseColor * aoFactor;
    
    // Pass to fragment shader
    FragPos = vec3(model * vec4(position, 1.0));
    Normal = mat3(model) * normal;
    Color = color;
    
    // Calculate final position
    gl_Position = projection * view * model * vec4(position, 1.0);
}