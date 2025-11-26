package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("your-secret-key")

func JWTMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {

        tokenString, err := c.Cookie("token")
        if err != nil || tokenString == "" {
            c.JSON(401, gin.H{"error": "Missing token"})
            c.Abort()
            return
        }

        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            return jwtSecret, nil
        })

        if err != nil || !token.Valid {
            c.JSON(401, gin.H{"error": "Invalid token"})
            c.Abort()
            return
        }

        claims := token.Claims.(jwt.MapClaims)

        c.Set("user_id", uint(claims["user_id"].(float64)))

        c.Next()
    }
}
