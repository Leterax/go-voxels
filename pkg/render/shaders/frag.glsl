#version 460 core
out vec4 FragColor;

in vec3 Normal;
in vec3 FragPos;
in vec3 Color;

uniform vec3 lightPos;
uniform vec3 lightColor;
uniform vec3 viewPos;

void main()
{
    // Normal rendering mode - use the vertex color with lighting
    vec3 objectColor = Color;

    // ambient
    float ambientStrength = 0.2;  
    vec3 ambient = ambientStrength * lightColor;

    // diffuse
    vec3 norm = normalize(Normal);
    vec3 lightDir = normalize(lightPos - FragPos);
    float diff = max(dot(norm, lightDir), 0.0);
    vec3 diffuse = diff * lightColor;

    // specular
    float specularStrength = 0.5;
    vec3 viewDir = normalize(viewPos - FragPos);
    vec3 reflectDir = reflect(-lightDir, norm);  
    float spec = pow(max(dot(viewDir, reflectDir), 0.0), 32);
    vec3 specular = specularStrength * spec * lightColor;  

    // combine
    vec3 result = (ambient + diffuse + specular) * objectColor;
    FragColor = vec4(result, 1.0); 
}