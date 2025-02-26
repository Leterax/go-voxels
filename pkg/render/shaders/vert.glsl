#version 460 core
layout (location = 0) in vec3 aPos;
layout (location = 1) in vec3 aNormal;
layout (location = 2) in vec3 aColor;

uniform mat4 model;
uniform mat4 view;
uniform mat4 projection;

out vec3 Normal;
out vec3 FragPos;
out vec3 Color;

void main()
{
    // Pass world-space position for lighting calculations
    FragPos = vec3(model * vec4(aPos, 1.0));
    
    // Transform normals to world space (properly handles non-uniform scaling)
    // Use 3x3 upper-left part of model matrix for normal transformation
    Normal = mat3(model) * aNormal;
    
    // Pass through color
    Color = aColor;
    
    // Transform vertex position
    gl_Position = projection * view * model * vec4(aPos, 1.0);
}